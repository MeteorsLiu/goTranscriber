package voice

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
)

// AudioInfo holds audio file information
type AudioInfo struct {
	SampleRate  int
	Channels    int
	SampleWidth int
	Samples     int64
}

// ffprobeOutput holds the JSON output from ffprobe
type ffprobeOutput struct {
	Streams []struct {
		SampleRate    string `json:"sample_rate"`
		Channels      int    `json:"channels"`
		SampleFmt     string `json:"sample_fmt"`
		DurationTS    int64  `json:"duration_ts"`
		Duration      string `json:"duration"`
		BitsPerSample int    `json:"bits_per_sample"`
	} `json:"streams"`
}

func exists(cmd string) (string, bool) {
	path, err := exec.LookPath(cmd)
	if err != nil {
		fmt.Println(err)
		return "", false
	}
	return path, true
}

// getAudioInfo uses ffprobe to get audio file information
func getAudioInfo(filename string) (*AudioInfo, error) {
	ffprobe, ok := exists("ffprobe")
	if !ok {
		return nil, errors.New("please install ffmpeg (ffprobe)")
	}

	cmd := exec.Command(ffprobe, "-v", "quiet", "-print_format", "json", "-show_streams", filename)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe failed: %w", err)
	}

	var probe ffprobeOutput
	if err := json.Unmarshal(output, &probe); err != nil {
		return nil, fmt.Errorf("failed to parse ffprobe output: %w", err)
	}

	if len(probe.Streams) == 0 {
		return nil, errors.New("no audio stream found")
	}

	stream := probe.Streams[0]

	sampleRate, err := strconv.Atoi(stream.SampleRate)
	if err != nil {
		return nil, fmt.Errorf("invalid sample rate: %w", err)
	}

	// Convert bits_per_sample to bytes (sampleWidth)
	sampleWidth := stream.BitsPerSample / 8
	if sampleWidth == 0 {
		// Default to 16-bit (2 bytes) if not available
		sampleWidth = 2
	}

	// Calculate total samples from duration
	var samples int64
	if stream.DurationTS > 0 {
		samples = stream.DurationTS
	} else if stream.Duration != "" {
		duration, err := strconv.ParseFloat(stream.Duration, 64)
		if err == nil {
			samples = int64(duration * float64(sampleRate))
		}
	}

	return &AudioInfo{
		SampleRate:  sampleRate,
		Channels:    stream.Channels,
		SampleWidth: sampleWidth,
		Samples:     samples,
	}, nil
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
	if ffmpeg, ok := exists("ffmpeg"); ok {
		audio, err := os.CreateTemp("", "*.pcm")
		if err != nil {
			return "", err
		}

		fmt.Println("Vad", audio.Name())
		// Only generate PCM file: 16000Hz, mono, 16-bit PCM
		cmd := exec.Command(ffmpeg, "-y", "-i", filename, "-ar", "16000", "-ac", "1", "-f", "s16le", "-acodec", "pcm_s16le", audio.Name())
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
		start_ := strconv.FormatFloat(max(0, start-0.25), 'f', -1, 64)
		duration := strconv.FormatFloat(end-start+0.25, 'f', -1, 64)

		cmd := exec.Command(cmd, "-y", "-ss", start_, "-t", duration, "-i", filename, "-ar", "16000", "-ac", "1", "-f", "s16le", "-acodec", "pcm_s16le", audio.Name())

		ret, err := cmd.CombinedOutput()
		if err != nil {
			return "", errors.New(string(ret))
		}
		return audio.Name(), nil
	}
	return "", errors.New("please install ffmpeg")
}
