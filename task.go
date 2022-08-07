package main

import (
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/MeteorsLiu/goSRT/srt"
	"github.com/MeteorsLiu/goSRT/transcribe"
	"github.com/MeteorsLiu/goSRT/voice"
)

var (
	t *transcribe.Transcriber
	s = sync.Pool{
		New: func() any {
			return new(srt.SRT)
		},
	}
)

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

func getSrtName(filename string) string {
	fn := filepath.Base(filename)
	dir := filepath.Dir(filename)
	prefix := strings.Split(fn, ".")[0]
	return filepath.Join(dir, prefix+".srt")
}

func Do(lang, filename string) {

	t = transcribe.New(lang, false)
	v := voice.New(filename, false)
	if v == nil {
		log.Fatal("Video Instance exits")
	}
	subrip := s.Get().(*srt.SRT)
	subrip.Reset()
	defer func() {
		v.Close()
		v = nil
		s.Put(subrip)
	}()
	//var wg sync.WaitGroup
	//var lock sync.Mutex
	//trans := map[int]string{}
	regions := v.Regions()
	if len(regions) == 0 {
		log.Println("unknown regions " + filename)
		return
	}
	log.Println("Start to transcribe the video")
	var sortedSubtitle []string
	slices := v.To(regions)
	log.Println("Slices Done")
	for index, file := range slices {
		// Pause the new goroutine until all goroutines are release
		/*
			wg.Add(1)
			go func() {
				defer wg.Done()
				subtitle, err := t.Transcribe(file)
				if err != nil {
					log.Printf("ID: %d error occurs: %v", index, err)
					return
				}
				log.Println(subtitle)
				lock.Lock()
				defer lock.Unlock()
				trans[index] = subtitle
			}()*/
		subtitle, err := t.Transcribe(file)
		if err != nil {
			log.Printf("ID: %d error occurs: %v", index, err)
		}
		log.Println(subtitle)
		sortedSubtitle = append(sortedSubtitle, subtitle)
	}
	//wg.Wait()
	log.Println("Transcribe Done.Waiting to sort the subtitle")
	// sort the map
	/*
		keys := make([]int, len(trans))
		sortedSubtitle := make([]string, len(trans))
		for k := range trans {
			keys = append(keys, k)
		}
		sort.Ints(keys)

		for _, s := range keys {
			sortedSubtitle = append(sortedSubtitle, trans[s])
		}*/
	ret := zip(regions, sortedSubtitle)
	for r, s := range ret {
		subrip.Append(strconv.FormatFloat(r.Start, 'f', -1, 64),
			strconv.FormatFloat(r.End, 'f', -1, 64),
			s)
	}
	if err := os.WriteFile(getSrtName(filename), []byte(subrip.String()), 0755); err != nil {
		log.Printf("Generating Subrip File Failed: %v", err)
	}
}

func DoVad(lang, filename string) {
	t = transcribe.New(lang, true)

	v := voice.New(filename, true)
	if v == nil {
		log.Fatal("Video Instance exits")
	}
	subrip := s.Get().(*srt.SRT)
	subrip.Reset()
	defer func() {
		v.Close()
		v = nil
		s.Put(subrip)
	}()
	//var wg sync.WaitGroup
	//var lock sync.Mutex
	//trans := map[int]string{}
	regions := v.Vad()
	if len(regions) == 0 || regions == nil {
		log.Println("unknown regions " + filename)
		return
	}
	log.Println("Start to transcribe the video")
	var sortedSubtitle []string
	slices := v.To(regions)
	log.Println("Slices Done")
	for index, file := range slices {
		// Pause the new goroutine until all goroutines are release
		/*
			wg.Add(1)
			go func() {
				defer wg.Done()
				subtitle, err := t.Transcribe(file)
				if err != nil {
					log.Printf("ID: %d error occurs: %v", index, err)
					return
				}
				log.Println(subtitle)
				lock.Lock()
				defer lock.Unlock()
				trans[index] = subtitle
			}()*/
		subtitle, err := t.Transcribe(file)
		if err != nil {
			log.Printf("ID: %d error occurs: %v", index, err)
		}
		log.Println(subtitle)
		sortedSubtitle = append(sortedSubtitle, subtitle)
	}
	//wg.Wait()
	log.Println("Transcribe Done.Waiting to sort the subtitle")
	// sort the map
	/*
		keys := make([]int, len(trans))
		sortedSubtitle := make([]string, len(trans))
		for k := range trans {
			keys = append(keys, k)
		}
		sort.Ints(keys)

		for _, s := range keys {
			sortedSubtitle = append(sortedSubtitle, trans[s])
		}*/
	ret := zip(regions, sortedSubtitle)
	for r, s := range ret {
		subrip.Append(strconv.FormatFloat(r.Start, 'f', -1, 64),
			strconv.FormatFloat(r.End, 'f', -1, 64),
			s)
	}
	if err := os.WriteFile(getSrtName(filename), []byte(subrip.String()), 0755); err != nil {
		log.Printf("Generating Subrip File Failed: %v", err)
	}
}
