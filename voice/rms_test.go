package voice

import (
	"github.com/MeteorsLiu/go-wav"
	"io"
	"os"
	"testing"
)

func TestRMS(t *testing.T) {
	file, _ := os.Open("/home/nfs/py/GVRD-94/1.wav")
	reader := wav.NewReader(file)
	defer file.Close()
	var sum float64
	for {
		samples, err := reader.ReadSamples(4096)
		if err == io.EOF {
			break
		}
		sum += rms(samples, 2)

	}
	t.Log(sum)

}
