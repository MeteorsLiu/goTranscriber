package voice

import (
	"math"
	"os"
	"strings"

	"github.com/MeteorsLiu/go-wav"
)

var (
	FRAME_WIDTH float64 = 4096.0
)

type Region struct {
	Start string
	End   string
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

}
