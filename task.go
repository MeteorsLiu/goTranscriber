package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/MeteorsLiu/goSRT/srt"
	"github.com/MeteorsLiu/goSRT/transcribe"
	"github.com/MeteorsLiu/goSRT/voice"
	gt "github.com/bas24/googletranslatefree"
	"github.com/jellyqwq/Paimon/webapi"
	"github.com/schollz/progressbar/v3"
	"golang.org/x/sync/semaphore"
)

var (
	t *transcribe.Transcriber
	s = sync.Pool{
		New: func() any {
			return new(srt.SRT)
		},
	}
)

type Subtitle struct {
	voice.Region
	Subtitle_String string
}

func srtNameOf(filename, vadMode string) string {
	fn := filepath.Base(filename)
	dir := filepath.Dir(filename)
	prefix := strings.Split(fn, ".")[0]

	srtname := filepath.Join(dir, prefix+".srt")

	if _, err := os.Stat(srtname); err == nil {
		tempFile, err := os.CreateTemp(dir, fmt.Sprintf("%s_%s_*.srt", prefix, vadMode))
		if err != nil {
			panic(err)
		}
		tempFile.Close()

		srtname = tempFile.Name()
	}

	return srtname
}

func DoVad(numConcurrent int, needTranslate bool, lang, filename, vadMode string) {
	isChina := transcribe.IsChina()
	t = transcribe.New(lang)

	// 选择VAD模式
	mode := voice.VadModeWebRTC
	if vadMode == "energy" {
		mode = voice.VadModeEnergy
		log.Println("Using Energy-based VAD (autosub method)")
	} else if vadMode == "webrtcpause" {
		mode = voice.VadModeWebRTCWithPause
		log.Println("Using WebRTC VAD With Pause Analysis")
	} else {
		log.Println("Using WebRTC VAD (default)")
	}

	v, err := voice.NewWithMode(filename, mode)
	if err != nil {
		log.Fatal(err)
	}
	subrip := s.Get().(*srt.SRT)
	subrip.Reset()
	defer func() {
		v.Close()
		v = nil
		s.Put(subrip)
	}()
	var wg sync.WaitGroup
	var lock sync.Mutex
	sema := semaphore.NewWeighted(int64(numConcurrent))

	trans := map[int]Subtitle{}
	regions := v.Vad()
	if len(regions) == 0 || regions == nil {
		log.Println("unknown regions " + filename)
		return
	}
	log.Println("Start to transcribe the video")

	slices := v.To(regions)
	log.Println("Slices Done")
	log.Println("Start to upload the video slices")
	bar := progressbar.Default(int64(len(slices)))

	for index, file := range slices {
		sema.Acquire(context.TODO(), 1)
		wg.Add(1)

		id := index
		file := file

		go func() {
			defer wg.Done()
			defer sema.Release(1)
			defer bar.Add(1)

			subtitle, err := t.Transcribe(file, true)

			if err != nil {
				if !errors.Is(err, transcribe.MAYBE_RETRY) {
					log.Printf("ID: %d error occurs: %v", id, err)
				}
				return
			}

			if needTranslate {
				var ts string
				if isChina {
					ts, err = webapi.RranslateByYouDao(subtitle)
				} else {
					ts, err = gt.Translate(subtitle, "auto", "zh-CN")
				}
				if err == nil {
					subtitle = ts
				}
			}

			lock.Lock()
			trans[id] = Subtitle{
				Region:          regions[id],
				Subtitle_String: subtitle,
			}
			lock.Unlock()
		}()
	}
	wg.Wait()

	log.Println("Transcribe Done.Waiting to sort the subtitle")

	var keys []int
	for k := range trans {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	for _, k := range keys {
		start := strconv.FormatFloat(trans[k].Region.Start, 'f', -1, 64)
		end := strconv.FormatFloat(trans[k].Region.End, 'f', -1, 64)
		log.Println(start, end)
		subrip.Append(start, end, trans[k].Subtitle_String)
	}
	if err := os.WriteFile(srtNameOf(filename, vadMode), []byte(subrip.String()), 0755); err != nil {
		log.Printf("Generating Subrip File Failed: %v", err)
	}
}
