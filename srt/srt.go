package srt

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type SRT struct {
	builder strings.Builder
	count   uint64
}

// formatTime function converts the internal time to SubRip Time
// for example 137.99 -> 00:02:17,990
func formatTime(internalTime string) (string, error) {
	_parseTime := strings.Split(internalTime, ".")
	if len(_parseTime) != 2 {
		return "", errors.New("unknown timestamp")
	}
	intTime, err := strconv.ParseInt(_parseTime[0], 10, 64)
	if err != nil {
		return "", err
	}
	milliTime := _parseTime[1]
	// Cut off the string if it's overflow.
	if len(milliTime) > 3 {
		milliTime = milliTime[:3]
	}
	// Padding to the length 3
	for len(milliTime) < 3 {
		milliTime += "0"
	}

	toConvertTime := time.Unix(intTime, 0)
	return fmt.Sprintf("%02d:%02d:%02d,%s",
		toConvertTime.Hour(),
		toConvertTime.Minute(),
		toConvertTime.Second(),
		milliTime), nil
}

func New() *SRT {
	return &SRT{
		count: 1,
	}
}

func (s *SRT) Append(start, end, content string) {
	s.builder.WriteString(strconv.FormatUint(s.count, 10))
	s.builder.WriteString("\n")
	start_, err1 := formatTime(start)
	_end, err2 := formatTime(end)
	if err1 != nil || err2 != nil {
		return
	}
	s.builder.WriteString(start_ + " --> " + _end)
	s.builder.WriteString("\n")
	s.builder.WriteString(content)
	s.builder.WriteString("\n")
	s.builder.WriteString("\n")
	s.count++
}

func (s *SRT) String() string {
	return s.builder.String()
}

func (s *SRT) Reset() {
	s.builder.Reset()
	s.count = 1
}
