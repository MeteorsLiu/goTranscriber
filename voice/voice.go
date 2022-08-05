package voice

import (
	"io"
	"math"
	"os"
	"strings"

	"github.com/MeteorsLiu/go-wav"
)

var (
	FRAME_WIDTH float64 = 4096.0
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
	if !strings.HasSuffix(filename, ".wav") {
		filename, err := extractAudio(filename)
		if err != nil {
			os.Remove(filename)
			return nil
		}
	}
	file, _ := os.Open(filename)
	reader := wav.NewReader(file)
	info, err := reader.Info()
	if err != nil {
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
	v.file.Close()
}

func (v *Voice) Regions() []Region {
	var energies []float64
	for i := 0; i < v.nChunks; i++ {
		samples, err := v.r.ReadSamples(4096)
		if err == io.EOF {
			break
		}
		r := rms(samples, v.nChannels)
		if r > 0 {
			energies = append(energies, r)
		}

	}
	threshold := percentile(energies, 0.2)
	var is_silence bool
	var max_exceeded bool
	var regions []Region
	var region_start float64
	var elapsed_time float64
	for _, energy := range energies {
		is_silence = energy <= threshold
		max_exceeded = region_start != 0 && (elapsed_time-region_start >= 6)
		if (max_exceeded || is_silence) && region_start != 0 {
			if elapsed_time-region_start >= 6 {
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
	return regions
}
