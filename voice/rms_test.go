package voice

import (
	"github.com/MeteorsLiu/go-wav"
	"os"
	"testing"
)

func TestRMS(t *testing.T) {
	file, _ := os.Open("/home/nfs/py/GVRD-94/1.wav")
	reader := wav.NewReader(file)
	defer file.Close()
	for {
		samples, err := reader.ReadSamples(4096)
		if err == io.EOF {
			break
		}
		t.Log(rms(samples))


	}

}
