package voice

import (
	"math"
	"sort"

	"github.com/MeteorsLiu/go-wav"
)

func percentile(arr []float64, percent float64) float64 {
	sort.Float64s(arr)
	index := float64(len(arr)-1) * percent
	floor := math.Floor(index)
	ceil := math.Ceil(index)
	if ceil == floor {
		return arr[int(index)]
	}
	low_value := arr[int(floor)] * (ceil - index)
	high_value := arr[int(ceil)] * (index - floor)
	return low_value + high_value
}

func rms(chunk []wav.Sample, nChannel int) float64 {
	var sumsq float64

	for _, sample := range chunk {
		for i := 0; i < nChannel; i++ {
			sumsq += float64(sample.Values[i] * sample.Values[i])
		}
	}
	return math.Sqrt(sumsq / FRAME_WIDTH)
}
