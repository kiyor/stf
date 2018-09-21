package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/wsxiaoys/terminal/color"
)

func LogHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t1 := time.Now()
		ctx := context.Background()
		writer := statusWriter{w, 0, 0}

		r = r.WithContext(ctx)
		next.ServeHTTP(&writer, r)

		range_ := r.Header.Get("Range")
		if len(range_) > 0 {
			range_ = range_[6:]
		} else {
			range_ = "-"
		}
		res := fmt.Sprintf("%v %v %v %v %v %v %v", r.RemoteAddr, writer.status, writer.length, r.Method, range_, r.Host+r.RequestURI, time.Since(t1))
		if *colors {
			log.Println(color.Sprintf("@{g}%s@{|}", res))
		} else {
			log.Println(res)
		}

	})
}
