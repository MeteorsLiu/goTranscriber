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
	"github.com/schollz/progressbar/v3"
)

var (
	FRAME_WIDTH     float64 = 4096.0
	MAX_REGION_SIZE float64 = 6.0
	MIN_REGION_SIZE float64 = 0.5
	MAX_CONCURRENT          = 10
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
}

func New(filename string) *Voice {

	f, err := extractAudio(filename)
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

	return &Voice{
		file:          file,
		r:             reader,
		rate:          info.FrameRate,
		nChannels:     info.NChannels,
		chunkDuration: FRAME_WIDTH / float64(info.FrameRate),
		nChunks:       int(math.Ceil(float64(info.NFrames) / FRAME_WIDTH)),
		sampleWidth:   info.SampleWidth,
	}
}

func (v *Voice) Close() {
	os.Remove(v.file.Name())
}

func (v *Voice) To(r []Region) []*os.File {
	var wg sync.WaitGroup
	var lock sync.Mutex
	file := map[int]*os.File{}
	count := 0
	bar := progressbar.Default(len(r))
	// Make sure the least context switching
	numConcurrent := runtime.NumCPU()
	for index, region := range r {
		// Pause the new goroutine until all goroutines are release
		if count >= numConcurrent {
			wg.Wait()
			count = 0
			// if the number of left elems is less than numConcurrent, reset the counter to 1.
			// make sure wg.Add() will not be paused
			if numConcurrent > 1 && (len(r)-index+1)-numConcurrent < 0 {
				numConcurrent = len(r) - index + 1
			}
		}
		if count == 0 {
			wg.Add(numConcurrent)
		}
		go func() {
			defer wg.Done()
			f, err := extractSlice(region.Start, region.End, v.file.Name())
			if err != nil {
				log.Println(err)
				return
			}
			lock.Lock()
			defer lock.Unlock()
			file[index] = f
			bar.Add(1)
		}()

		count++

	}
	if count >= 0 {
		wg.Wait()
	}

	// sort the map
	keys := make([]int, len(file))
	sortedFile := make([]*os.File, len(file))
	for k := range file {
		keys = append(keys, k)
	}
	sort.Ints(keys)
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
