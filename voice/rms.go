package voice

import (
	"math"

	"github.com/MeteorsLiu/go-wav"
)

func rms(chunk []wav.Sample) float64 {
	if len(chunk) > 4096 || len(chunk) == 0 {
		return 0
	}
	var sumsq float64
	for _, sample := range chunk {
		for _, val := range sample {
			sumsq += float64(val * val)
		}
	}
	return math.Sqrt(sumsq)
}
