package voice

import (
	"testing"
	"github.com/MeteorsLiu/go-wav"
	"os"
)
func TestRMS(t *testing.T) {
	var buf [4096]byte
	file, _ := os.Open("/home/nfs/py/GVRD-94/1.wav")
	reader := wav.NewReader(file)
	defer file.Close()
	reader.Read(buf)
	t.Log(rms(buf, 2))
}