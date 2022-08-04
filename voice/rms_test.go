package voice

import (
	"testing"
	"github.com/MeteorsLiu/go-wav"
	"os"
)
func TestRMS(t *testing.T) {
	buf := make([]byte, 4096)
	file, _ := os.Open("/home/nfs/py/GVRD-94/1.wav")
	reader := wav.NewReader(file)
	defer file.Close()
	reader.Read(buf)
	t.Log(rms(buf, 2))
}