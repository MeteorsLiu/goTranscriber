package voice

import "testing"

func TestVoice(t *testing.T) {
	f, err := New("test_test.mp3", true)
	if err != nil {
		t.Error("cannot read file", err)
		return
	}
	region := f.Vad()

	t.Log(f.To(region))
}
