package main

import (
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

func zip(a []voice.Region, b []string) map[voice.Region]string {
	regions := map[voice.Region]string{}
	if len(a) > len(b) {
		for i, s := range b {
			regions[a[i]] = s
		}
	} else {
		for i, s := range a {
			regions[s] = b[i]
		}
	}
	return regions
}

func srtNameOf(filename string) string {
	fn := filepath.Base(filename)
	dir := filepath.Dir(filename)
	prefix := strings.Split(fn, ".")[0]

	srtname := filepath.Join(dir, prefix+".srt")

	if _, err := os.Stat(srtname); err == nil {
		tempFile, err := os.CreateTemp(dir, fmt.Sprintf("%s_*.srt", prefix))
		if err != nil {
			panic(err)
		}
		tempFile.Close()

		srtname = tempFile.Name()
	}

	return srtname
}

func DoVad(needTranslate bool, lang, filename, vadMode string) {
	isChina := transcribe.IsChina()
	t = transcribe.New(lang)

	// 选择VAD模式
	mode := voice.VadModeWebRTC
	if vadMode == "energy" {
		mode = voice.VadModeEnergy
		log.Println("Using Energy-based VAD (autosub method)")
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
	numConcurrent := 30
	count := 0
	goid := make(chan int)
	fileCh := make(chan string)
	for index, _file := range slices {
		// Pause the new goroutine until all goroutines are release
		if count >= numConcurrent {
			wg.Wait()
			count = 0
		}

		wg.Add(1)

		go func() {
			defer wg.Done()
			id := <-goid
			file := <-fileCh
			subtitle, err := t.Transcribe(file, true)
			lock.Lock()
			defer func() {
				bar.Add(1)
				lock.Unlock()
			}()
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
				if err != nil {
					ts = subtitle
				}
				trans[id] = Subtitle{
					Region:          regions[id],
					Subtitle_String: ts,
				}
			} else {
				trans[id] = Subtitle{
					Region:          regions[id],
					Subtitle_String: subtitle,
				}
			}

		}()
		goid <- index
		fileCh <- _file
		count++
	}
	if count > 0 {
		wg.Wait()
	}
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
	if err := os.WriteFile(srtNameOf(filename), []byte(subrip.String()), 0755); err != nil {
		log.Printf("Generating Subrip File Failed: %v", err)
	}
}
