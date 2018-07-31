package main

import (
	"fmt"
	"math/rand"
	"mime"
	"net/http"
	"regexp"
	"strconv"
	"time"
)

func testFileHandler(w http.ResponseWriter, r *http.Request) {
	// 	qs := r.URL.Query()
	if reTestFile.MatchString(r.URL.Path) {
		iStr := reTestFile.FindStringSubmatch(r.URL.Path)[1]
		l, err := strconv.Atoi(iStr)
		if err != nil {
			return
		}
		s := reTestFile.FindStringSubmatch(r.URL.Path)[2]
		if len(reTestFile.FindStringSubmatch(r.URL.Path)) > 3 {
			ext := mime.TypeByExtension(reTestFile.FindStringSubmatch(r.URL.Path)[3])
			if len(ext) > 0 {
				w.Header().Set("Content-Type", ext)
			}
		}
		if len(r.Header.Get("X-Cache-Control")) > 0 {
			w.Header().Set("Cache-Control", r.Header.Get("X-Cache-Control"))
		} else {
			w.Header().Set("Cache-Control", *testFileCC)
		}
		params := r.URL.Query()
		for k, v := range params {
			w.Header().Set(k, v[0])
		}
		s1 := rand.NewSource(time.Now().UnixNano())
		r1 := rand.New(s1)
		bt := make([]byte, 1024)

		var rangeReq bool
		var start, end, contentSize, contentLength int
		switch s {
		case "b", "B":
			contentSize = l
		case "k", "K":
			contentSize = l * 1024
		case "m", "M":
			contentSize = l * 1024 * 1024
		case "g", "G":
			contentSize = l * 1024 * 1024 * 1024
		}
		if len(r.Header.Get("Range")) > 0 {
			reRange := regexp.MustCompile(`^bytes=(\d+)-(\d+)$`)
			if reRange.MatchString(r.Header.Get("Range")) {
				start_ := reRange.FindStringSubmatch(r.Header.Get("Range"))[1]
				end_ := reRange.FindStringSubmatch(r.Header.Get("Range"))[2]
				start, _ = strconv.Atoi(start_)
				end, _ = strconv.Atoi(end_)
				rangeReq = true
			}
			reRange = regexp.MustCompile(`^bytes=(\d+)-$`)
			if reRange.MatchString(r.Header.Get("Range")) {
				start_ := reRange.FindStringSubmatch(r.Header.Get("Range"))[1]
				start, _ = strconv.Atoi(start_)
				end = contentSize - 1
				rangeReq = true
			}
			contentLength = end - start + 1
		} else {
			contentLength = contentSize
		}
		w.Header().Set("Content-Length", strconv.Itoa(contentLength))
		w.Header().Set("Last-Modified", time.Now().Format(http.TimeFormat))
		if rangeReq {
			w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, contentSize))
			w.WriteHeader(206)
			b := make([]byte, contentLength)
			r1.Read(b)
			x, _ := w.Write(b)
			serveByte += uint64(x)
		} else {
			switch s {
			case "b", "B":
				b := make([]byte, l)
				r1.Read(b)
				x, _ := w.Write(b)
				serveByte += uint64(x)
			case "k", "K":
				for i := 0; i < l; i++ {
					r1.Read(bt)
					x, _ := w.Write(bt)
					serveByte += uint64(x)
				}
			case "m", "M":
				for i := 0; i < l; i++ {
					for j := 0; j < 1024; j++ {
						r1.Read(bt)
						x, _ := w.Write(bt)
						// 					d += uint64(x)
						serveByte += uint64(x)
					}
				}
			case "g", "G":
				for i := 0; i < l; i++ {
					for j := 0; j < 1024; j++ {
						for k := 0; k < 1024; k++ {
							r1.Read(bt)
							x, _ := w.Write(bt)
							// 						d += uint64(x)
							serveByte += uint64(x)
						}
					}
				}
			}
		}
	}
}
