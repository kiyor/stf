package utils

import (
	"bufio"
	"errors"
	"net"
	"net/http"
)

type StatusWriter struct {
	http.ResponseWriter
	status int
	length int
}

func (w *StatusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *StatusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = 200
	}
	w.length += len(b)
	return w.ResponseWriter.Write(b)
}

// Status returns the HTTP status code
func (w *StatusWriter) Status() int {
	return w.status
}

// Length returns the number of bytes written
func (w *StatusWriter) Length() int {
	return w.length
}

// Flush forwards to the underlying writer so streaming responses still flush.
func (w *StatusWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Hijack forwards to the underlying writer (needed for CONNECT/websocket).
func (w *StatusWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if h, ok := w.ResponseWriter.(http.Hijacker); ok {
		if w.status == 0 {
			w.status = http.StatusSwitchingProtocols
		}
		return h.Hijack()
	}
	return nil, nil, errors.New("hijack not supported")
}

// Unwrap lets http.ResponseController reach the underlying writer.
func (w *StatusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }
