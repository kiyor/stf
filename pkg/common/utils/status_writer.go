package utils

import "net/http"

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
