package voice

import (
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"runtime"
	"sort"
	"strings"
	"sync"

	"github.com/MeteorsLiu/go-wav"
	"github.com/baabaaox/go-webrtcvad"
	"github.com/schollz/progressbar/v3"
)

var (
	FRAME_WIDTH            float64 = 4096.0
	MAX_REGION_SIZE        float64 = 10.0 // 硬限制：强制切分
	SOFT_REGION_SIZE       float64 = 6.0  // 软限制：到达后在静音处切分
	MIN_REGION_SIZE        float64 = 0.5
	VAD_FRAME_DURATION_SEC float64 = 0.02
	MAX_CONCURRENT                 = 10
	VAD_MODE                       = 1
	VAD_TOLERANCE_FRAMES           = 10 // 容忍帧(200ms)的静音，避免BGM间隙导致碎片化
)

type Region struct {
	Start float64
	End   float64
}
type Voice struct {
	file          *os.File
	r             *wav.Reader
	videofile     string
	rate          int
	nChannels     int
	chunkDuration float64
	nChunks       int
	sampleWidth   int
	isVad         bool
}

func New(filename string, isVad bool) (*Voice, error) {
	var f string
	var err error
	if isVad {
		f, err = extractVadAudio(filename)
	} else {
		f, err = extractAudio(filename)
	}

	if err != nil {
		return nil, err
	}

	fmt.Println(f)

	file, err := os.Open(f)
	if err != nil {
		log.Println("Failed to open audio file:", err)
		return nil, err
	}

	fmt.Println("open")

	reader := wav.NewReader(file)
	info, err := reader.Info()
	if err != nil {
		log.Println("Failed to read audio info:", err)
		file.Close()
		return nil, err
	}

	var chunkDuration float64
	if isVad {
		// VAD使用20ms帧
		chunkDuration = VAD_FRAME_DURATION_SEC // 0.02秒
	} else {
		chunkDuration = FRAME_WIDTH / float64(info.FrameRate)
	}
	vfile := filename
	// wmv may cause pcm convert problem.
	if strings.HasSuffix(vfile, ".wmv") {
		vfile = f
	}

	return &Voice{
		file:          file,
		r:             reader,
		videofile:     vfile,
		isVad:         isVad,
		rate:          info.FrameRate,
		nChannels:     info.NChannels,
		chunkDuration: chunkDuration,
		nChunks:       int(math.Ceil(float64(info.NFrames) / FRAME_WIDTH)),
		sampleWidth:   info.SampleWidth,
	}, nil
}

func (v *Voice) Close() {
	log.Println("Remove tmp file" + v.file.Name())
	os.Remove(v.file.Name())
}

func (v *Voice) To(r []Region) []string {
	var lock sync.Mutex
	var wg sync.WaitGroup

	file := map[int]string{}
	goid := make(chan int)
	regionCh := make(chan Region)
	bar := progressbar.Default(int64(len(r)))

	// Make sure the least context switching
	numConcurrent := runtime.NumCPU()
	count := 0
	for index, _region := range r {
		// Pause the new goroutine until all goroutines are release
		if count >= numConcurrent {
			wg.Wait()
			count = 0
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			id := <-goid
			region := <-regionCh
			var f string
			var err error
			if v.isVad {
				f, err = extractVadSlice(region.Start, region.End, v.videofile)
			} else {
				f, err = extractSlice(region.Start, region.End, v.file.Name())
			}

			if err != nil {
				log.Println(err)
				return
			}
			lock.Lock()
			defer lock.Unlock()
			defer bar.Add(1)
			file[id] = f
		}()
		goid <- index
		regionCh <- _region
		count++
	}
	if count > 0 {
		wg.Wait()
	}

	// sort the map
	var keys []int
	var sortedFile []string
	for k := range file {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	//log.Println(keys)
	for _, i := range keys {
		sortedFile = append(sortedFile, file[i])
	}
	return sortedFile
}
func (v *Voice) Regions() []Region {
	var energies []float64
	for i := 0; i < v.nChunks; i++ {
		samples, err := v.r.ReadSamples(4096)
		if err == io.EOF {
			break
		}
		energies = append(energies, rms(samples, v.nChannels))
	}
	threshold := percentile(energies, 0.2)
	var is_silence bool
	var max_exceeded bool
	var regions []Region
	var region_start float64
	var elapsed_time float64
	for _, energy := range energies {
		is_silence = energy <= threshold
		max_exceeded = region_start != 0 && (elapsed_time-region_start >= MAX_REGION_SIZE)
		if (max_exceeded || is_silence) && region_start != 0 {
			if elapsed_time-region_start >= MIN_REGION_SIZE {
				regions = append(regions, Region{
					Start: region_start,
					End:   elapsed_time,
				})
				region_start = 0
			}
		} else if region_start == 0 && !is_silence {
			region_start = elapsed_time
		}
		elapsed_time += v.chunkDuration
	}
	// tell gc to sweep the mem. no more need
	v.r = nil
	return regions
}

func (v *Voice) Vad() []Region {
	if v.rate != 16000 && v.rate != 32000 && v.rate != 48000 {
		log.Fatal("error audio frame rate")
	}

	// 重新定位到文件开头
	v.file.Seek(0, 0)

	// WebRTC VAD 帧大小计算 (20ms)
	frameBytes := v.rate / 50 * v.sampleWidth * v.nChannels // 20ms = 1/50秒
	frameBuffer := make([]byte, frameBytes)
	frameSize := v.rate / 50 // 每帧样本数

	// 初始化VAD
	vadInst := webrtcvad.Create()
	defer webrtcvad.Free(vadInst)
	webrtcvad.Init(vadInst)

	err := webrtcvad.SetMode(vadInst, VAD_MODE)
	if err != nil {
		log.Fatal(err)
	}

	var regions []Region
	var currentRegionStart float64
	var isInRegion bool
	var silenceFrameCount int // 连续静音帧计数
	var needsSoftSplit bool   // 是否需要在静音处软切分
	frameTime := 0.02         // 20ms per frame
	currentTime := 0.0

	for {
		n, err := v.file.Read(frameBuffer)
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Printf("Error reading frame: %v", err)
			break
		}
		if n < frameBytes {
			break // 不完整的帧，结束处理
		}

		// 检测当前帧是否包含语音
		hasVoice, err := webrtcvad.Process(vadInst, v.rate, frameBuffer, frameSize)
		if err != nil {
			log.Printf("VAD process error: %v", err)
			hasVoice = false
		}

		if hasVoice {
			// 重置静音计数
			silenceFrameCount = 0
			if !isInRegion {
				// 开始新的语音区域
				currentRegionStart = currentTime
				isInRegion = true
				needsSoftSplit = false
			}
		} else {
			if isInRegion {
				// 增加静音帧计数
				silenceFrameCount++

				// 如果已标记需要软切分，且检测到静音，则在此处切分
				if needsSoftSplit && silenceFrameCount > VAD_TOLERANCE_FRAMES {
					regionEnd := currentTime - float64(VAD_TOLERANCE_FRAMES)*frameTime
					regionDuration := regionEnd - currentRegionStart
					if regionDuration >= MIN_REGION_SIZE {
						regions = append(regions, Region{
							Start: currentRegionStart,
							End:   regionEnd,
						})
					}
					isInRegion = false
					silenceFrameCount = 0
					needsSoftSplit = false
				} else if !needsSoftSplit && silenceFrameCount > VAD_TOLERANCE_FRAMES {
					// 正常的静音结束
					regionEnd := currentTime - float64(VAD_TOLERANCE_FRAMES)*frameTime
					regionDuration := regionEnd - currentRegionStart
					if regionDuration >= MIN_REGION_SIZE {
						regions = append(regions, Region{
							Start: currentRegionStart,
							End:   regionEnd,
						})
					}
					isInRegion = false
					silenceFrameCount = 0
				}
			}
		}

		// 检查区域长度（软硬限制）
		if isInRegion {
			regionDuration := currentTime - currentRegionStart

			// 达到硬限制，强制切分
			if regionDuration >= MAX_REGION_SIZE {
				regions = append(regions, Region{
					Start: currentRegionStart,
					End:   currentTime,
				})
				currentRegionStart = currentTime
				silenceFrameCount = 0
				needsSoftSplit = false
			} else if regionDuration >= SOFT_REGION_SIZE && !needsSoftSplit {
				// 达到软限制，标记在下次静音时切分
				needsSoftSplit = true
			}
		}

		currentTime += frameTime
	}

	// 处理文件结束时仍在进行的区域
	if isInRegion {
		regionEnd := currentTime - float64(silenceFrameCount)*frameTime
		regionDuration := regionEnd - currentRegionStart
		if regionDuration >= MIN_REGION_SIZE {
			regions = append(regions, Region{
				Start: currentRegionStart,
				End:   regionEnd,
			})
		}
	}

	// tell gc to sweep the mem. no more need
	v.r = nil
	return regions
}
