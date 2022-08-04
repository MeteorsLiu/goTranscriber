package voice

import (
	"encoding/binary"
	"math"
)

func rms16(chunk []byte, width int) float64 {
	var sumsq float64
	for i := 0; i < 4096; i += width {
		sumsq += math.Pow(float64(binary.LittleEndian.Uint16(chunk[i:width])), 2)
	}
	return math.Sqrt(sumsq)
}
func rms32(chunk []byte, width int) float64 {
	var sumsq float64
	for i := 0; i < 4096; i += width {
		sumsq += math.Pow(float64(binary.LittleEndian.Uint32(chunk[i:width])), 2)
	}
	return math.Sqrt(sumsq)
}

func rms(chunk []byte, width int) float64 {
	if len(chunk) > 4096 {
		return 0
	}
	var rmsRet float64
	switch width {
	case 1, 2:
		rmsRet = rms16(chunk, width)
	default:
		rmsRet = rms32(chunk, width)
	}
	return rmsRet
}
