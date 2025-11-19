package voice

import (
	"io"
	"log"
	"os"
	"runtime"
	"sort"
	"strings"
	"sync"

	"github.com/baabaaox/go-webrtcvad"
	"github.com/schollz/progressbar/v3"
)

var (
	FRAME_WIDTH            float64 = 4096.0
	MAX_REGION_SIZE        float64 = 10.0 // 硬限制：强制切分
	MIN_REGION_SIZE        float64 = 0.5
	VAD_FRAME_DURATION_SEC float64 = 0.02
	MAX_CONCURRENT                 = 10
	VAD_MODE                       = 2
	SOFT_REGION_SIZE       float64 = 4.5 // 软限制：超过此长度考虑在停顿处切分

	// VAD停顿检测参数（帧数，20ms/帧）
	IGNORE_PAUSE_FRAMES = 10 // 200ms - 忽略的短停顿
	MEDIUM_PAUSE_FRAMES = 20 // 400ms - 中等停顿，软限制时可切分
	LONG_PAUSE_FRAMES   = 35 // 700ms - 长停顿，总是切分

	// 切片参数
	REGION_OVERLAP = 0.25 // 区域重叠时间（秒），避免硬切分把词切断
)

type Region struct {
	Start float64
	End   float64
}

type Voice struct {
	file      *os.File
	videofile string
}

func New(filename string) (*Voice, error) {
	// wmv may cause pcm convert problem.
	if strings.HasSuffix(filename, ".wmv") {
		wavFile, err := extractWavAudio(filename)
		if err != nil {
			return nil, err
		}
		filename = wavFile
	}

	pcmFileName, err := extractVadAudio(filename)
	if err != nil {
		return nil, err
	}

	pcmFile, err := os.Open(pcmFileName)
	if err != nil {
		log.Println("Failed to open PCM file:", err)
		return nil, err
	}

	return &Voice{
		file:      pcmFile,
		videofile: filename,
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

	numConcurrent := runtime.NumCPU()
	count := 0
	for index, _region := range r {
		if count >= numConcurrent {
			wg.Wait()
			count = 0
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			id := <-goid
			region := <-regionCh

			f, err := extractVadSlice(region.Start, region.End, v.videofile)
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

	var keys []int
	var sortedFile []string
	for k := range file {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	for _, i := range keys {
		sortedFile = append(sortedFile, file[i])
	}
	return sortedFile
}

// Vad 两步走：先VAD识别区域，再用频谱分析过滤BGM
func (v *Voice) Vad() []Region {
	// 步骤1: VAD识别所有声音区域
	rawRegions := v.detectVoiceRegions()
	log.Printf("VAD detected %d raw regions", len(rawRegions))

	// // 步骤2: 频谱分析过滤BGM
	// refinedRegions := v.refineRegionsWithSpectral(rawRegions)
	// log.Printf("Spectral analysis refined to %d regions", len(refinedRegions))

	return rawRegions
}

// detectVoiceRegions 使用VAD识别声音区域
func (v *Voice) detectVoiceRegions() []Region {
	v.file.Seek(0, 0)

	frameBytes := 16000 / 50 * 2
	frameBuffer := make([]byte, frameBytes)
	frameSize := 16000 / 50
	frameTime := 0.02

	vadInst := webrtcvad.Create()
	defer webrtcvad.Free(vadInst)
	webrtcvad.Init(vadInst)
	webrtcvad.SetMode(vadInst, VAD_MODE)

	var regions []Region
	var currentRegionStart float64
	var isInRegion bool
	currentTime := 0.0
	silenceFrames := 0 // 连续无声帧计数

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
			break
		}

		hasVoice, err := webrtcvad.Process(vadInst, 16000, frameBuffer, frameSize)
		if err != nil {
			log.Printf("VAD process error: %v", err)
			hasVoice = false
		}

		if hasVoice {
			if !isInRegion {
				currentRegionStart = currentTime
				isInRegion = true
			}
			silenceFrames = 0
		} else {
			if isInRegion {
				silenceFrames++
				regionDuration := currentTime - currentRegionStart

				// 检查是否应该在停顿处切分
				shouldSplit := false

				// 长停顿：总是切分（>700ms）
				if silenceFrames >= LONG_PAUSE_FRAMES {
					shouldSplit = true
					log.Printf("Long pause: %d frames at %.2fs", silenceFrames, currentTime)
				} else if silenceFrames >= MEDIUM_PAUSE_FRAMES && regionDuration >= SOFT_REGION_SIZE {
					// 中等停顿：如果已超过软限制则切分（>400ms且区域>4.5s）
					shouldSplit = true
					log.Printf("Medium pause with soft limit: %d frames at %.2fs (dur %.2fs)",
						silenceFrames, currentTime, regionDuration)
				}

				if shouldSplit {
					// 在停顿开始处切分，当前区域End延长0.25秒，下一个区域Start提前0.25秒
					splitPoint := currentTime - float64(silenceFrames)*frameTime
					if splitPoint-currentRegionStart >= MIN_REGION_SIZE {
						regions = append(regions, Region{
							Start: currentRegionStart,
							End:   splitPoint + REGION_OVERLAP, // 延长0.25秒避免切断词
						})
						log.Printf("VAD split: %.2f-%.2f (%.2fs)",
							currentRegionStart, splitPoint+REGION_OVERLAP, splitPoint+REGION_OVERLAP-currentRegionStart)
					}
					// 下一个区域从停顿点往前0.25秒开始（保留重叠）
					nextStart := splitPoint - REGION_OVERLAP
					if nextStart < 0 {
						nextStart = 0
					}
					currentRegionStart = nextStart
					isInRegion = false
					silenceFrames = 0
				}
			}
		}

		// 硬限制：超过最大长度强制切分
		if isInRegion && (currentTime-currentRegionStart) >= MAX_REGION_SIZE {
			regions = append(regions, Region{
				Start: currentRegionStart,
				End:   currentTime + REGION_OVERLAP, // 延长0.25秒避免切断词
			})
			log.Printf("Hard limit: %.2f-%.2f (%.2fs)",
				currentRegionStart, currentTime+REGION_OVERLAP, currentTime+REGION_OVERLAP-currentRegionStart)
			// 下一个区域从当前时间往前0.25秒开始（保留重叠）
			nextStart := currentTime - REGION_OVERLAP
			if nextStart < 0 {
				nextStart = 0
			}
			currentRegionStart = nextStart
		}

		currentTime += frameTime
	}

	// 处理文件结束时的区域
	if isInRegion {
		regionDuration := currentTime - currentRegionStart
		if regionDuration >= MIN_REGION_SIZE {
			regions = append(regions, Region{
				Start: currentRegionStart,
				End:   currentTime,
			})
		}
	}

	return regions
}
