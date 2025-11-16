package voice

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
)

func exists(cmd string) (string, bool) {
	path, err := exec.LookPath(cmd)
	if err != nil {
		fmt.Println(err)
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
		fmt.Println("Vad", audio.Name())
		cmd := exec.Command(cmd, "-y", "-i", filename, "-ar", "16000", "-ac", "1", "-acodec", "pcm_s16le", audio.Name())
		ret, err := cmd.CombinedOutput()
		if err != nil {
			return "", errors.New(string(ret))
		}
		return audio.Name(), nil
	}
	return "", errors.New("please install ffmpeg")
}

func extractSlice(start, end float64, filename string) (string, error) {
	if cmd, ok := exists("ffmpeg"); ok {
		audio, err := os.CreateTemp("", "*.pcm")
		if err != nil {
			return "", err
		}
		defer audio.Close()
		start_ := strconv.FormatFloat(start, 'f', -1, 64)
		duration := strconv.FormatFloat(end-start, 'f', -1, 64)
		cmd := exec.Command(cmd, "-y", "-ss", start_, "-t", duration, "-i", filename, "-acodec", "pcm_s16le", audio.Name())
		ret, err := cmd.CombinedOutput()
		if err != nil {
			return "", errors.New(string(ret))
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
		start_ := strconv.FormatFloat(start, 'f', -1, 64)
		duration := strconv.FormatFloat(end-start, 'f', -1, 64)

		cmd := exec.Command(cmd, "-y", "-ss", start_, "-t", duration, "-i", filename, "-ar", "16000", "-ac", "1", "-f", "s16le", "-acodec", "pcm_s16le", audio.Name())

		ret, err := cmd.CombinedOutput()
		if err != nil {
			return "", errors.New(string(ret))
		}
		return audio.Name(), nil
	}
	return "", errors.New("please install ffmpeg")
}
