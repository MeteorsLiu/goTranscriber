package voice

import (
	"io"
	"log"
	"math"
	"os"
	"runtime"
	"sort"
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
	VAD_MODE                       = 0 // WebRTC VAD模式: 0=质量(推荐), 1=低比特率, 2=积极, 3=非常积极

	// WebRTC VAD 参数
	MIN_SILENCE_FRAMES = 5 // 100ms - 连续静音帧数阈值，避免瞬时误判

	// 切片参数
	REGION_OVERLAP = 0.25 // 区域重叠时间（秒），避免硬切分把词切断

	// Energy-based VAD 参数 (autosub方法)
	ENERGY_THRESHOLD_PERCENTILE = 0.2 // 使用能量的20%百分位作为阈值
)

// VadMode 定义VAD检测模式
type VadMode string

const (
	VadModeWebRTC          VadMode = "webrtc" // WebRTC VAD (默认)
	VadModeWebRTCWithPause VadMode = "webrtcpause"
	VadModeEnergy          VadMode = "energy" // 基于能量的VAD (autosub方法)
)

type Region struct {
	Start float64
	End   float64
}

type Voice struct {
	file      *os.File
	videofile string
	vadMode   VadMode // VAD检测模式
}

func New(filename string) (*Voice, error) {
	return NewWithMode(filename, VadModeWebRTC)
}

func NewWithMode(filename string, mode VadMode) (*Voice, error) {
	// wmv may cause pcm convert problem.
	wavFile, err := extractWavAudio(filename)
	if err != nil {
		return nil, err
	}
	filename = wavFile

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
		vadMode:   mode,
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

// Vad 根据设置的模式选择VAD方法
func (v *Voice) Vad() []Region {
	var rawRegions []Region

	switch v.vadMode {
	case VadModeEnergy:
		// 使用基于能量的VAD (autosub方法)
		rawRegions = v.detectVoiceRegionsByEnergy()
		log.Printf("Energy-based VAD detected %d regions", len(rawRegions))
	case VadModeWebRTCWithPause:
		rawRegions = v.detectVoiceRegionsWithPause()
		log.Printf("WebRTC VAD With Pause Analysis detected %d regions", len(rawRegions))
	default:
		// 使用WebRTC VAD (默认)
		rawRegions = v.detectVoiceRegions()
		log.Printf("WebRTC VAD detected %d regions", len(rawRegions))
	}

	// // 可选：频谱分析过滤BGM
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

		// 简化逻辑：参考 Energy VAD
		// 1. 检测到连续静音（>= MIN_SILENCE_FRAMES）就切分
		// 2. 超过 MAX_REGION_SIZE 强制切分

		if hasVoice {
			if !isInRegion {
				currentRegionStart = currentTime
				isInRegion = true
			}
			silenceFrames = 0
		} else {
			if isInRegion {
				silenceFrames++

				// 连续静音达到阈值，结束当前区域
				if silenceFrames >= MIN_SILENCE_FRAMES {
					splitPoint := currentTime - float64(silenceFrames)*frameTime
					if splitPoint-currentRegionStart >= MIN_REGION_SIZE {
						regions = append(regions, Region{
							Start: currentRegionStart,
							End:   splitPoint,
						})
						log.Printf("Silence split: %.2f-%.2f (%.2fs, %d silent frames)",
							currentRegionStart, splitPoint, splitPoint-currentRegionStart, silenceFrames)
					}
					isInRegion = false
					silenceFrames = 0
				}
			}
		}

		// 硬限制：超过最大长度强制切分
		if isInRegion && (currentTime-currentRegionStart) >= MAX_REGION_SIZE {
			regions = append(regions, Region{
				Start: currentRegionStart,
				End:   currentTime,
			})
			log.Printf("Hard limit: %.2f-%.2f (%.2fs)",
				currentRegionStart, currentTime, currentTime-currentRegionStart)
			isInRegion = false
			silenceFrames = 0
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

	// 统一添加 0.25 秒偏移避免切断词
	for i := range regions {
		regions[i].Start -= REGION_OVERLAP
		if regions[i].Start < 0 {
			regions[i].Start = 0
		}
		regions[i].End += REGION_OVERLAP
	}

	return regions
}

// detectVoiceRegionsByEnergy 使用能量检测方法识别声音区域（autosub方法）
func (v *Voice) detectVoiceRegionsByEnergy() []Region {
	v.file.Seek(0, 0)

	// 读取整个音频文件
	data, err := io.ReadAll(v.file)
	if err != nil {
		log.Printf("Error reading audio file: %v", err)
		return nil
	}

	// PCM 16-bit, 16kHz, mono
	sampleRate := 16000
	frameWidth := int(FRAME_WIDTH)
	chunkDuration := float64(frameWidth) / float64(sampleRate)

	// 计算能量
	numChunks := len(data) / (frameWidth * 2) // 2 bytes per sample
	if numChunks == 0 {
		return nil
	}

	energies := make([]float64, 0, numChunks)
	for i := 0; i < numChunks; i++ {
		startByte := i * frameWidth * 2
		endByte := startByte + frameWidth*2
		if endByte > len(data) {
			break
		}
		chunk := data[startByte:endByte]
		energy := calculateRMS(chunk)
		energies = append(energies, energy)
	}

	// 计算阈值（20%百分位）
	threshold := percentile(energies, ENERGY_THRESHOLD_PERCENTILE)
	log.Printf("Energy threshold: %.2f (from %d chunks)", threshold, len(energies))

	// 识别语音区域
	var regions []Region
	var regionStart float64
	var inRegion bool
	elapsedTime := 0.0

	for _, energy := range energies {
		isSilence := energy <= threshold
		maxExceeded := inRegion && (elapsedTime-regionStart) >= MAX_REGION_SIZE

		if (maxExceeded || isSilence) && inRegion {
			// 结束当前区域
			if elapsedTime-regionStart >= MIN_REGION_SIZE {
				// 添加0.25秒偏移量避免切断词
				start := regionStart - REGION_OVERLAP
				if start < 0 {
					start = 0
				}
				end := elapsedTime + REGION_OVERLAP

				regions = append(regions, Region{
					Start: start,
					End:   end,
				})
				log.Printf("Energy region: %.2f-%.2f (%.2fs)", start, end, end-start)
			}
			inRegion = false
		} else if !inRegion && !isSilence {
			// 开始新区域
			regionStart = elapsedTime
			inRegion = true
		}

		elapsedTime += chunkDuration
	}

	// 处理文件结束时的区域
	if inRegion && (elapsedTime-regionStart) >= MIN_REGION_SIZE {
		// 添加0.25秒偏移量避免切断词
		start := regionStart - REGION_OVERLAP
		if start < 0 {
			start = 0
		}
		end := elapsedTime + REGION_OVERLAP

		regions = append(regions, Region{
			Start: start,
			End:   end,
		})
	}

	return regions
}

// detectVoiceRegions 使用VAD识别声音区域
func (v *Voice) detectVoiceRegionsWithPause() []Region {
	v.file.Seek(0, 0)

	frameBytes := 16000 / 50 * 2
	frameBuffer := make([]byte, frameBytes)
	frameSize := 16000 / 50
	frameTime := 0.02

	vadInst := webrtcvad.Create()
	defer webrtcvad.Free(vadInst)
	webrtcvad.Init(vadInst)
	webrtcvad.SetMode(vadInst, 2)

	var regions []Region
	var currentRegionStart float64
	var isInRegion bool
	currentTime := 0.0
	silenceFrames := 0 // 连续无声帧计数

	const (
		SOFT_REGION_SIZE float64 = 6 // 软限制：超过此长度考虑在停顿处切分

		// VAD停顿检测参数（帧数，20ms/帧）
		IGNORE_PAUSE_FRAMES = 10 // 200ms - 忽略的短停顿
		MEDIUM_PAUSE_FRAMES = 20 // 400ms - 中等停顿，软限制时可切分
		LONG_PAUSE_FRAMES   = 35 // 700ms - 长停顿，总是切分
	)

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
							End:   splitPoint, // 延长0.25秒避免切断词
						})
						log.Printf("VAD split: %.2f-%.2f (%.2fs)",
							currentRegionStart, splitPoint, splitPoint-currentRegionStart)
					}
					// 下一个区域从停顿点往前0.25秒开始（保留重叠）
					nextStart := splitPoint
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
				End:   currentTime, // 延长0.25秒避免切断词
			})
			log.Printf("Hard limit: %.2f-%.2f (%.2fs)",
				currentRegionStart, currentTime, currentTime-currentRegionStart)
			// 下一个区域从当前时间往前0.25秒开始（保留重叠）
			nextStart := currentTime
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

	// 统一添加 0.25 秒偏移避免切断词
	for i := range regions {
		regions[i].Start -= REGION_OVERLAP
		if regions[i].Start < 0 {
			regions[i].Start = 0
		}
		regions[i].End += REGION_OVERLAP
	}

	return regions
}

// calculateRMS 计算音频块的RMS（均方根）能量
func calculateRMS(pcmData []byte) float64 {
	if len(pcmData) == 0 {
		return 0
	}

	// PCM 16-bit little-endian
	numSamples := len(pcmData) / 2
	var sum float64

	for i := 0; i < numSamples; i++ {
		// 读取16-bit little-endian sample
		sample := int16(pcmData[i*2]) | (int16(pcmData[i*2+1]) << 8)
		sum += float64(sample) * float64(sample)
	}

	return math.Sqrt(sum / float64(numSamples))
}

// percentile 计算给定百分位的值
func percentile(values []float64, percent float64) float64 {
	if len(values) == 0 {
		return 0
	}

	// 复制并排序
	sorted := make([]float64, len(values))
	copy(sorted, values)
	sort.Float64s(sorted)

	// 计算索引
	index := (float64(len(sorted)) - 1) * percent
	floor := int(index)
	ceil := floor + 1

	if ceil >= len(sorted) {
		return sorted[floor]
	}

	// 线性插值
	lowValue := sorted[floor] * (float64(ceil) - index)
	highValue := sorted[ceil] * (index - float64(floor))

	return lowValue + highValue
}
