package voice

import "testing"
func TestVoice(t *testing.T) {
	f := New("/home/nfs/py/GVRD-94/1.wav")
	if f == nil {
		t.Error("cannot read file")
		return
	}
	t.Log(f.Regions())
}