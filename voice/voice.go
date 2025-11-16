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
	MAX_REGION_SIZE        float64 = 6.0
	MIN_REGION_SIZE        float64 = 0.5
	VAD_FRAME_DURATION_SEC float64 = 0.02
	MAX_CONCURRENT                 = 10
	VAD_FRAME_DURATION             = 20
	VAD_MODE                       = 0
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
	frameTime := 0.02 // 20ms per frame
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
			if !isInRegion {
				// 开始新的语音区域
				currentRegionStart = currentTime
				isInRegion = true
			}
		} else {
			if isInRegion {
				// 结束当前语音区域
				regionDuration := currentTime - currentRegionStart
				if regionDuration >= MIN_REGION_SIZE {
					regions = append(regions, Region{
						Start: currentRegionStart,
						End:   currentTime,
					})
				}
				isInRegion = false
			}
		}

		// 检查是否超过最大区域长度
		if isInRegion && (currentTime-currentRegionStart) >= MAX_REGION_SIZE {
			regions = append(regions, Region{
				Start: currentRegionStart,
				End:   currentTime,
			})
			currentRegionStart = currentTime
		}

		currentTime += frameTime
	}

	// 处理文件结束时仍在进行的区域
	if isInRegion {
		regionDuration := currentTime - currentRegionStart
		if regionDuration >= MIN_REGION_SIZE {
			regions = append(regions, Region{
				Start: currentRegionStart,
				End:   currentTime,
			})
		}
	}

	// tell gc to sweep the mem. no more need
	v.r = nil
	return regions
}
