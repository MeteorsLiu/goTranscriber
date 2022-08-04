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
	samples, _ := reader.ReadSamples(4096)
	t.Log(rms(samples), len(samples))
}
