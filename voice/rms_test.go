package voice

import (
	"github.com/MeteorsLiu/go-wav"
	"io"
	"os"
	"testing"
)

func TestRMS(t *testing.T) {
	buf := make([]byte, 4096)
	file, _ := os.Open("/home/nfs/py/GVRD-94/1.wav")
	reader := wav.NewReader(file)
	defer file.Close()
	io.ReadFull(reader, buf)
	t.Log(rms(buf, 2))
}
