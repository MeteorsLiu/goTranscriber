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
		if err := exec.Command(cmd, "-y", "-i", filename, "-ar", "44100", "-ac", "1", audio.Name()).Run(); err != nil {
			return "", err
		}
		return audio.Name(), nil
	}
	return "", errors.New("please install ffmpeg")
}

func extractVadAudio(filename string) (string, error) {
	if cmd, ok := exists("ffmpeg"); ok {
		audio, err := os.CreateTemp("", "*.wav")
		if err != nil {
			return "", err
		}
		if err := exec.Command(cmd, "-y", "-i", filename, "-ar", "16000", "-ac", "1", "-acodec", "pcm_s16le", audio.Name()).Run(); err != nil {
			return "", err
		}
		return audio.Name(), nil
	}
	return "", errors.New("please install ffmpeg")
}

func extractSlice(start, end float64, filename string) (string, error) {
	if cmd, ok := exists("ffmpeg"); ok {
		audio, err := os.CreateTemp("", "*.wav")
		if err != nil {
			return "", err
		}
		defer audio.Close()
		start_ := strconv.FormatFloat(start+0.25, 'f', -1, 64)
		_end := strconv.FormatFloat(end-start, 'f', -1, 64)
		if err := exec.Command(cmd, "-y", "-ss", start_, "-t", _end, "-i", filename, "-acodec", "pcm_s16le", audio.Name()).Run(); err != nil {
			return "", err
		}
		return audio.Name(), nil
	}
	return "", errors.New("please install ffmpeg")
}

func extractVadSlice(start, end float64, filename string) (string, error) {
	if cmd, ok := exists("ffmpeg"); ok {
		audio, err := os.CreateTemp("", "*.pcm")
		if err != nil {
			return "", err
		}
		defer audio.Close()
		start_ := strconv.FormatFloat(start+0.25, 'f', -1, 64)
		_end := strconv.FormatFloat(end-start, 'f', -1, 64)
		if err := exec.Command(cmd, "-y", "-ss", start_, "-t", _end, "-i", filename, "-ar", "16000", "-ac", "1", "-f", "s16le", "-acodec", "pcm_s16le", audio.Name()).Run(); err != nil {
			return "", err
		}
		return audio.Name(), nil
	}
	return "", errors.New("please install ffmpeg")
}
