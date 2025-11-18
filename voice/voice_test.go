package voice

import (
	"os"
	"os/exec"
	"testing"
)

func TestVoice(t *testing.T) {
	f, err := New("test_test.mp3", true)
	if err != nil {
		t.Error("cannot read file", err)
		return
	}
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
		cmd := exec.Command("ffplay", "-autoexit", "-ar", "16000", "-f", "s16le", "-acodec", "pcm_s16le", file)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Run()
	}
}
