package voice

import "testing"

func TestVoice(t *testing.T) {
	f := New("/home/nfs/py/GVRD-94/GVRD-94_01.mkv", true)
	if f == nil {
		t.Error("cannot read file")
		return
	}
	defer f.Close()
	t.Log(f.Vad())
}
