package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	m, _ := filepath.Glob("/home/nfs/py/[A-Z]*/[A-Z]*.mp4")
	for _, ms := range m {
		if _, err := os.Stat(getSrtName(ms)); errors.Is(err, os.ErrNotExist) {
			fmt.Println(ms)

			DoVad("ja", ms)
		}
	}
}
