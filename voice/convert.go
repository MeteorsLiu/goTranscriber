package voice

import (
	"errors"
	"os"
	"os/exec"
	"strconv"
)

func exists(cmd string) (string, bool) {
	path, err := exec.LookPath(cmd)
	if err != nil {
		return "", false
	}
	return path, true
}

func extractAudio(filename string) (string, error) {
	if cmd, ok := exists("ffmpeg"); ok {
		audio, err := os.CreateTemp("", "*.wav")
		if err != nil {
			return "", err
		}
		if err := exec.Command(cmd, "-i", filename, "-ar", "44100", "-ac", "1", audio.Name()).Run(); err != nil {
			return "", err
		}
		return audio.Name(), nil
	}
	return "", errors.New("please install ffmpeg")
}

func extractSlice(start, end, filename string) ([]byte, error) {
	if cmd, ok := exists("ffmpeg"); ok {
		audio, err := os.CreateTemp("", "*.wav")
		if err != nil {
			return nil, err
		}
		defer os.Remove(audio.Name())
		floatStart, err1 := strconv.ParseFloat(start, 64)
		floatEnd, err2 := strconv.ParseFloat(end, 64)
		if err1 != nil {
			return nil, err1
		}
		if err2 != nil {
			return nil, err2
		}
		start_ := strconv.FormatFloat(floatStart+0.25, 'f', -1, 64)
		_end := strconv.FormatFloat(floatEnd-floatStart, 'f', -1, 64)
		if err := exec.Command(cmd, "-ss", start_, "-t", _end, "-i", filename, audio.Name()).Run(); err != nil {
			return nil, err
		}
		return os.ReadFile(audio.Name())
	}
	return nil, errors.New("please install ffmpeg")
}