package util

import (
	"os"
	"log"
	"strings"
)

func LoadAuthToken(fname string) string {
	f, err := os.Open(fname)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	buf := make([]byte, 256)
	n, err := f.Read(buf)
	if err != nil {
		log.Fatal(err)
	}
	return strings.TrimSpace(string(buf[:n]))
}
