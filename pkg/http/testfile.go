package http

import (
	"fmt"
	"io"
	"math/rand"
	"mime"
	"net/http"
	"regexp"
	"strconv"
	"sync/atomic"
	"time"
)

var (
	reTestFile   = regexp.MustCompile(`(\d+)(b|B|k|K|m|M|g|G)(.*)`)
	reRangeFull  = regexp.MustCompile(`^bytes=(\d+)-(\d+)$`)
	reRangeStart = regexp.MustCompile(`^bytes=(\d+)-$`)
	serveByte    uint64 // total bytes served; update with atomic.AddUint64
)

// maxTestFileSize caps generated test files (1 TiB) so size math cannot overflow.
const maxTestFileSize = int64(1) << 40

// TestFileHandler serves a generated file of random bytes whose size comes from
// the URL, e.g. /10M or /512k.bin. Data is streamed, never fully buffered.
// Query parameters are copied into response headers (intended, for CDN tests).
func TestFileHandler(w http.ResponseWriter, r *http.Request, testFileCC string) {
	m := reTestFile.FindStringSubmatch(r.URL.Path)
	if m == nil {
		http.Error(w, "usage: /<n>{b,k,m,g}[.ext]", http.StatusNotFound)
		return
	}
	l, err := strconv.ParseInt(m[1], 10, 64)
	if err != nil {
		http.Error(w, "size too large", http.StatusRequestedRangeNotSatisfiable)
		return
	}
	var unit int64 = 1
	switch m[2] {
	case "k", "K":
		unit = 1 << 10
	case "m", "M":
		unit = 1 << 20
	case "g", "G":
		unit = 1 << 30
	}
	if l > maxTestFileSize/unit {
		http.Error(w, "size too large", http.StatusRequestedRangeNotSatisfiable)
		return
	}
	contentSize := l * unit

	if ext := mime.TypeByExtension(m[3]); len(ext) > 0 {
		w.Header().Set("Content-Type", ext)
	}
	if len(r.Header.Get("X-Cache-Control")) > 0 {
		w.Header().Set("Cache-Control", r.Header.Get("X-Cache-Control"))
	} else {
		w.Header().Set("Cache-Control", testFileCC)
	}
	for k, v := range r.URL.Query() {
		w.Header().Set(k, v[0])
	}

	start, end := int64(0), contentSize-1
	rangeReq := false
	if rh := r.Header.Get("Range"); rh != "" {
		if rm := reRangeFull.FindStringSubmatch(rh); rm != nil {
			start, _ = strconv.ParseInt(rm[1], 10, 64)
			end, _ = strconv.ParseInt(rm[2], 10, 64)
			rangeReq = true
		} else if rm := reRangeStart.FindStringSubmatch(rh); rm != nil {
			start, _ = strconv.ParseInt(rm[1], 10, 64)
			rangeReq = true
		}
		if rangeReq {
			if end >= contentSize {
				end = contentSize - 1
			}
			if start < 0 || start > end {
				w.Header().Set("Content-Range", fmt.Sprintf("bytes */%d", contentSize))
				http.Error(w, "range not satisfiable", http.StatusRequestedRangeNotSatisfiable)
				return
			}
		}
	}
	contentLength := end - start + 1
	if contentLength < 0 {
		contentLength = 0
	}

	w.Header().Set("Content-Length", strconv.FormatInt(contentLength, 10))
	w.Header().Set("Last-Modified", time.Now().UTC().Format(http.TimeFormat))
	if rangeReq {
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, contentSize))
		w.WriteHeader(http.StatusPartialContent)
	}
	if r.Method == http.MethodHead {
		return
	}
	src := rand.New(rand.NewSource(time.Now().UnixNano()))
	n, _ := io.CopyN(w, src, contentLength)
	atomic.AddUint64(&serveByte, uint64(n))
}
