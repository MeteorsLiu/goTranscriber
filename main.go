package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/MeteorsLiu/goSRT/transcribe"
)

var (
	lang     string
	filename string
)

func main() {
	flag.StringVar(&lang, "lang", "", "Source Video Language(源文件语言)")
	flag.StringVar(&filename, "file", "", "Source Video(原视频文件)")
	flag.Parse()

	if lang == "" || filename == "" {
		fmt.Println("具体使用方式：")
		fmt.Println("./gotranscriber -file xxx.mp4(File to be transcribed. 需要听识的视频/音频文件) -lang ja(原视频文件语言缩写)")
		fmt.Println("以下是语言简写")
		for k, v := range transcribe.GetLangCode() {
			fmt.Println(k, "->", v)
		}
		os.Exit(0)
	}

	DoVad(lang, filename)

}
