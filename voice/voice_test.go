package voice

import (
	"os/exec"
	"testing"
)

func TestVoice(t *testing.T) {
	f, err := New("test_test.mp3", true)
	if err != nil {
		t.Error("cannot read file", err)
		return
	}
	f.file.Name()
	region := f.Vad()

	t.Log(f.To(region))
}

func TestVoiceWithReduce(t *testing.T) {
	f, err := New("/Users/haolan/Downloads/SPSD-02.mp4", true)
	if err != nil {
		t.Error("cannot read file", err)
		return
	}

	for _, file := range f.To(f.Vad()) {
		exec.Command("ffplay", "-autoexit", "-ar", "16000", "-f", "s16le", "-acodec", "pcm_s16le", file).Run()
	}
}
