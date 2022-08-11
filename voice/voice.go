package voice

import (
	"io"
	"log"
	"math"
	"os"
	"runtime"
	"sort"
	"sync"

	"github.com/MeteorsLiu/go-wav"
	"github.com/baabaaox/go-webrtcvad"
	"github.com/schollz/progressbar/v3"
)

var (
	FRAME_WIDTH            float64 = 4096.0
	MAX_REGION_SIZE        float64 = 8.0
	MIN_REGION_SIZE        float64 = 1.0
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
	rate          int
	nChannels     int
	chunkDuration float64
	nChunks       int
	sampleWidth   int
	isVad         bool
}

func New(filename string, isVad bool) *Voice {
	var f string
	var err error
	if isVad {
		f, err = extractVadAudio(filename)
	} else {
		f, err = extractAudio(filename)
	}

	if err != nil {
		log.Println(err)
		os.Remove(f)
		return nil
	}

	file, _ := os.Open(f)
	reader := wav.NewReader(file)
	info, err := reader.Info()
	if err != nil {
		log.Println(err)
		return nil
	}
	var chunkDuration float64
	if isVad {
		WIDTH := info.FrameRate / 1000 * VAD_FRAME_DURATION * 16 / 8
		chunkDuration = (float64(WIDTH) / float64(info.FrameRate)) / 2.0
	} else {
		chunkDuration = FRAME_WIDTH / float64(info.FrameRate)
	}

	return &Voice{
		file:          file,
		r:             reader,
		isVad:         isVad,
		rate:          info.FrameRate,
		nChannels:     info.NChannels,
		chunkDuration: chunkDuration,
		nChunks:       int(math.Ceil(float64(info.NFrames) / FRAME_WIDTH)),
		sampleWidth:   info.SampleWidth,
	}
}

func (v *Voice) Close() {
	log.Println("Remove tmp file" + v.file.Name())
	os.Remove(v.file.Name())
}

func (v *Voice) To(r []Region) []*os.File {
	var lock sync.Mutex
	file := map[int]*os.File{}

	bar := progressbar.Default(int64(len(r)))
	// Make sure the least context switching
	goid := make(chan int)
	regionCh := make(chan Region)
	var wg sync.WaitGroup
	numConcurrent := runtime.NumCPU()
	count := 0
	for index, _region := range r {
		// Pause the new goroutine until all goroutines are release
		if count >= numConcurrent {
			wg.Wait()
			count = 0
			/*
				if len(r)-index+1-numConcurrent < 0 && numConcurrent > 1 {
					numConcurrent = 1
				}*/
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			id := <-goid
			region := <-regionCh
			var f *os.File
			var err error
			if v.isVad {
				f, err = extractVadSlice(region.Start, region.End, v.file.Name())
			} else {
				f, err = extractSlice(region.Start, region.End, v.file.Name())
			}

			if err != nil {
				log.Println(err)
				return
			}
			lock.Lock()
			defer lock.Unlock()
			file[id] = f
			bar.Add(1)
		}()
		goid := <-index
		regionCh <- _region
		count++
	}
	if count > 0 {
		wg.Wait()
	}

	// sort the map
	var keys []int
	var sortedFile []*os.File
	for k := range file {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	log.Println(keys)
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
	// tell gc to sweep the mem. no more need
	v.r = nil
	WIDTH := v.rate / 1000 * VAD_FRAME_DURATION * 16 / 8
	frameBuffer := make([]byte, WIDTH)
	frameSize := v.rate / 1000 * VAD_FRAME_DURATION
	vadInst := webrtcvad.Create()
	defer webrtcvad.Free(vadInst)
	webrtcvad.Init(vadInst)

	err := webrtcvad.SetMode(vadInst, VAD_MODE)
	if err != nil {
		log.Fatal(err)
	}
	var region_start float64
	var elapsed_time float64
	var regions []Region
	for {
		_, err = v.file.Read(frameBuffer)
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Println(err)
			return nil
		}
		frameActive, _ := webrtcvad.Process(vadInst, v.rate, frameBuffer, frameSize)
		if (elapsed_time-region_start >= MAX_REGION_SIZE || !frameActive) && region_start != 0 {
			if elapsed_time-region_start >= MIN_REGION_SIZE {
				regions = append(regions, Region{
					Start: region_start,
					End:   elapsed_time,
				})
				region_start = 0
			}
		} else if region_start == 0 && frameActive {
			region_start = elapsed_time
		}
		elapsed_time += v.chunkDuration
	}
	return regions
}
