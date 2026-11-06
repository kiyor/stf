package middleware

import (
	"fmt"
	"log"
	"net/http"
	"strings"
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
		writer := &utils.StatusWriter{ResponseWriter: w}

		// Keep the request's own context so client disconnects cancel work.
		next.ServeHTTP(writer, r)

		range_ := r.Header.Get("Range")
		if len(range_) > 0 {
			range_ = strings.TrimPrefix(range_, "bytes=")
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
