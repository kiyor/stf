package middleware

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/kiyor/stf/pkg/common/utils"
	"github.com/wsxiaoys/terminal/color"
)

// LogHandler creates a logging middleware
func LogHandler(next http.Handler) http.Handler {
	return LogHandlerWithColors(next, false)
}

// LogHandlerWithColors creates a logging middleware with optional colors
func LogHandlerWithColors(next http.Handler, useColors bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t1 := time.Now()
		ctx := context.Background()
		writer := &utils.StatusWriter{ResponseWriter: w}

		r = r.WithContext(ctx)
		next.ServeHTTP(writer, r)

		range_ := r.Header.Get("Range")
		if len(range_) > 0 {
			range_ = range_[6:]
		} else {
			range_ = "-"
		}
		res := fmt.Sprintf("%v %v %v %v %v %v %v", r.RemoteAddr, writer.Status(), writer.Length(), r.Method, range_, r.Host+r.RequestURI, time.Since(t1))
		if useColors {
			log.Println(color.Sprintf("@{g}%s@{|}", res))
		} else {
			log.Println(res)
		}
	})
}
