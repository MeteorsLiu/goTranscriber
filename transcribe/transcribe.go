package transcribe

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

const (
	RETRY_TIMES       = 3
	KEY               = "AIzaSyBOti4mM-6x9WDnZIjIeyEU21OpBXqWBgw"
	GOOGLE_COMMON_URL = "http://www.google.com/speech-api/v2/recognize?client=chromium&lang=%s&key=%s"
	GOOGLE_CN_URL     = "http://www.google.cn/speech-api/v2/recognize?client=chromium&lang=%s&key=%s"
)

var (
	MAYBE_RETRY = errors.New("retry")
)

type Transcriber struct {
	bufPool sync.Pool
	url     string
}

func GetLangCode() map[string]string {
	lang := map[string]string{}
	lang["af"] = "Afrikaans"
	lang["ar"] = "Arabic"
	lang["az"] = "Azerbaijani"
	lang["be"] = "Belarusian"
	lang["bg"] = "Bulgarian"
	lang["bn"] = "Bengali"
	lang["bs"] = "Bosnian"
	lang["ca"] = "Catalan"
	lang["ceb"] = "Cebuano"
	lang["cs"] = "Czech"
	lang["cy"] = "Welsh"
	lang["da"] = "Danish"
	lang["de"] = "German"
	lang["el"] = "Greek"
	lang["en-AU"] = "English (Australia)"
	lang["en-CA"] = "English (Canada)"
	lang["en-GB"] = "English (United Kingdom)"
	lang["en-IN"] = "English (India)"
	lang["en-IE"] = "English (Ireland)"
	lang["en-NZ"] = "English (New Zealand)"
	lang["en-PH"] = "English (Philippines)"
	lang["en-SG"] = "English (Singapore)"
	lang["en-US"] = "English (United States)"
	lang["eo"] = "Esperanto"
	lang["es-AR"] = "Spanish (Argentina)"
	lang["es-CL"] = "Spanish (Chile)"
	lang["es-ES"] = "Spanish (Spain)"
	lang["es-US"] = "Spanish (United States)"
	lang["es-MX"] = "Spanish (Mexico)"
	lang["es"] = "Spanish"
	lang["et"] = "Estonian"
	lang["eu"] = "Basque"
	lang["fa"] = "Persian"
	lang["fi"] = "Finnish"
	lang["fr"] = "French"
	lang["ga"] = "Irish"
	lang["gl"] = "Galician"
	lang["gu"] = "Gujarati"
	lang["ha"] = "Hausa"
	lang["hi"] = "Hindi"
	lang["hmn"] = "Hmong"
	lang["hr"] = "Croatian"
	lang["ht"] = "Haitian Creole"
	lang["hu"] = "Hungarian"
	lang["hy"] = "Armenian"
	lang["id"] = "Indonesian"
	lang["ig"] = "Igbo"
	lang["is"] = "Icelandic"
	lang["it"] = "Italian"
	lang["iw"] = "Hebrew"
	lang["ja"] = "Japanese"
	lang["jw"] = "Javanese"
	lang["ka"] = "Georgian"
	lang["kk"] = "Kazakh"
	lang["km"] = "Khmer"
	lang["kn"] = "Kannada"
	lang["ko"] = "Korean"
	lang["la"] = "Latin"
	lang["lo"] = "Lao"
	lang["lt"] = "Lithuanian"
	lang["lv"] = "Latvian"
	lang["mg"] = "Malagasy"
	lang["mi"] = "Maori"
	lang["mk"] = "Macedonian"
	lang["ml"] = "Malayalam"
	lang["mn"] = "Mongolian"
	lang["mr"] = "Marathi"
	lang["ms"] = "Malay"
	lang["mt"] = "Maltese"
	lang["my"] = "Myanmar (Burmese)"
	lang["ne"] = "Nepali"
	lang["nl"] = "Dutch"
	lang["no"] = "Norwegian"
	lang["ny"] = "Chichewa"
	lang["pa"] = "Punjabi"
	lang["pl"] = "Polish"
	lang["pt-BR"] = "Portuguese (Brazil)"
	lang["pt-PT"] = "Portuguese (Portugal)"
	lang["ro"] = "Romanian"
	lang["ru"] = "Russian"
	lang["si"] = "Sinhala"
	lang["sk"] = "Slovak"
	lang["sl"] = "Slovenian"
	lang["so"] = "Somali"
	lang["sq"] = "Albanian"
	lang["sr"] = "Serbian"
	lang["st"] = "Sesotho"
	lang["su"] = "Sudanese"
	lang["sv"] = "Swedish"
	lang["sw"] = "Swahili"
	lang["ta"] = "Tamil"
	lang["te"] = "Telugu"
	lang["tg"] = "Tajik"
	lang["th"] = "Thai"
	lang["tl"] = "Filipino"
	lang["tr"] = "Turkish"
	lang["uk"] = "Ukrainian"
	lang["ur"] = "Urdu"
	lang["uz"] = "Uzbek"
	lang["vi"] = "Vietnamese"
	lang["yi"] = "Yiddish"
	lang["yo"] = "Yoruba"
	lang["yue-Hant-HK"] = "Cantonese (Traditional HK)"
	lang["zh"] = "Chinese (Simplified China)"
	lang["zh-HK"] = "Chinese (Simplified Hong Kong)"
	lang["zh-TW"] = "Chinese (Traditional Taiwan)"
	lang["zu"] = "Zulu"
	return lang
}

func getMyIP() string {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", "http://whatismyip.akamai.com", nil)
	if err != nil {
		log.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil && err != io.EOF {
		log.Fatal(err)
	}
	return string(body)
}
func isChina() bool {
	ip := getMyIP()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("https://api.ip.sb/geoip/%s", ip), nil)
	if err != nil {
		log.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil && err != io.EOF {
		log.Fatal(err)
	}
	ret := map[string]interface{}{}
	if err := json.Unmarshal(body, &ret); err != nil {
		log.Fatal(err)
	}
	if code, ok := ret["country_code"]; ok {
		if code.(string) == "CN" {
			return true
		}
	}
	return false
}

func New(lang string) *Transcriber {
	if _, ok := GetLangCode()[lang]; !ok {
		log.Fatal("error language code")
	}
	var url string
	/*
		if isChina() {
			url = fmt.Sprintf(GOOGLE_CN_URL, lang, KEY)
		} else {

		}*/
	url = fmt.Sprintf(GOOGLE_COMMON_URL, lang, KEY)
	return &Transcriber{
		bufPool: sync.Pool{
			New: func() any {
				return new(bytes.Buffer)
			},
		},
		url: url,
	}
}

func (t *Transcriber) transcribe(buf *bytes.Buffer, isVad bool) (string, error) {
	defer func() {
		// Don't let it panic
		_ = recover()
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "POST", t.url, buf)
	if err != nil {
		return "", err
	}
	req.Header.Set("user-agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/102.0.0.0 Safari/537.36")
	if isVad {
		req.Header.Set("Content-Type", "audio/l16; rate=16000;")
	} else {
		req.Header.Set("Content-Type", "audio/l16; rate=44100;")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	//log.Println(string(res))
	var ret map[string][]interface{}
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		ret = map[string][]interface{}{}
		_ = json.Unmarshal(scanner.Bytes(), &ret)
		//log.Println(ret)
		if result, ok := ret["result"]; ok {
			if len(result) == 0 {
				continue
			}
			return ret["result"][0].(map[string]interface{})["alternative"].([]interface{})[0].(map[string]interface{})["transcript"].(string), nil
		}
	}
	return "", MAYBE_RETRY
}

func doRetry(call func() (string, error)) (string, error) {
	var err error
	var ret string
	init := time.Second
	for i := 0; i < RETRY_TIMES; i++ {
		<-time.After(init)
		ret, err = call()
		if err == nil {
			return ret, nil
		}
		init = time.Duration(int(2^(i+1)/2)) * time.Second
	}
	return ret, err
}
func (t *Transcriber) Transcribe(file *os.File, isVad bool) (string, error) {
	if file == nil {
		return "", errors.New("nil pointer")
	}
	buf := t.bufPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer t.bufPool.Put(buf)
	defer os.Remove(file.Name())
	_, err := io.Copy(buf, file)
	if err != nil {
		return "", err
	}
	var ret string
	ret, err = t.transcribe(buf, isVad)
	if err != nil {
		if errors.Is(err, MAYBE_RETRY) {
			ret, err = doRetry(func() (string, error) {
				return t.transcribe(buf, isVad)
			})
		}
		if err != nil {
			return "", err
		}
	}
	// Maybe it panic
	if ret == "" {
		return "", errors.New("transcribe panic")
	}
	return ret, nil

}
