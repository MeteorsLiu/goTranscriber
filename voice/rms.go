package voice

import (
	"math"

	"github.com/MeteorsLiu/go-wav"
)

func rms(chunk []wav.Sample) float64 {
	var sumsq float64
	for _, sample := range chunk {
		for i := 0; i < 2; i++ {
			sumsq += float64(sample.Values[i] * sample.Values[i])
		}
	}
	return math.Sqrt(sumsq)
}
