package http

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// testFileCC for testing
const defaultTestFileCC = "public,max-age=60"

func TestTestFileHandler(t *testing.T) {
	tests := []struct {
		name           string
		path           string
		wantStatus     int
		wantHeaders    map[string]string
		wantMinSize    int
		expectResponse bool
	}{
		{
			name:           "1KB file",
			path:           "/1k",
			wantStatus:     http.StatusOK,
			wantHeaders:    map[string]string{"Content-Length": "1024", "Cache-Control": defaultTestFileCC},
			wantMinSize:    1024,
			expectResponse: true,
		},
		{
			name:           "500B file",
			path:           "/500b",
			wantStatus:     http.StatusOK,
			wantHeaders:    map[string]string{"Content-Length": "500"},
			wantMinSize:    500,
			expectResponse: true,
		},
		{
			name:           "1MB file",
			path:           "/1m",
			wantStatus:     http.StatusOK,
			wantHeaders:    map[string]string{"Content-Length": "1048576"},
			wantMinSize:    1048576,
			expectResponse: true,
		},
		{
			name:           "file with extension",
			path:           "/1k.json",
			wantStatus:     http.StatusOK,
			wantHeaders:    map[string]string{"Content-Type": "application/json"},
			expectResponse: true,
		},
		{
			name:           "non-matching path",
			path:           "/invalid",
			wantStatus:     http.StatusOK,
			expectResponse: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset serveByte counter
			serveByte = 0

			req := httptest.NewRequest("GET", tt.path, nil)
			w := httptest.NewRecorder()

			TestFileHandler(w, req, defaultTestFileCC)

			resp := w.Result()

			if tt.expectResponse {
				if resp.StatusCode != tt.wantStatus {
					t.Errorf("TestFileHandler() status = %v, want %v", resp.StatusCode, tt.wantStatus)
				}

				for header, expectedValue := range tt.wantHeaders {
					if got := resp.Header.Get(header); got != expectedValue {
						t.Errorf("TestFileHandler() header %v = %v, want %v", header, got, expectedValue)
					}
				}

				if tt.wantMinSize > 0 {
					body := w.Body.Bytes()
					if len(body) < tt.wantMinSize {
						t.Errorf("TestFileHandler() body size = %v, want >= %v", len(body), tt.wantMinSize)
					}
				}
			}
		})
	}
}

func TestTestFileHandlerWithRange(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		rangeHeader string
		wantStatus  int
		wantLength  int
	}{
		{
			name:        "range request bytes=0-499",
			path:        "/1k",
			rangeHeader: "bytes=0-499",
			wantStatus:  http.StatusPartialContent,
			wantLength:  500,
		},
		{
			name:        "range request bytes=100-",
			path:        "/1k",
			rangeHeader: "bytes=100-",
			wantStatus:  http.StatusPartialContent,
			wantLength:  924, // 1024 - 100
		},
		{
			name:        "invalid range format",
			path:        "/1k",
			rangeHeader: "bytes=invalid",
			wantStatus:  http.StatusOK, // Should fall back to full response
			wantLength:  1024,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			serveByte = 0

			req := httptest.NewRequest("GET", tt.path, nil)
			req.Header.Set("Range", tt.rangeHeader)
			w := httptest.NewRecorder()

			TestFileHandler(w, req, defaultTestFileCC)

			resp := w.Result()

			if resp.StatusCode != tt.wantStatus {
				t.Errorf("TestFileHandler() status = %v, want %v", resp.StatusCode, tt.wantStatus)
			}

			body := w.Body.Bytes()
			if len(body) != tt.wantLength {
				t.Errorf("TestFileHandler() body length = %v, want %v", len(body), tt.wantLength)
			}

			if tt.wantStatus == http.StatusPartialContent {
				if resp.Header.Get("Content-Range") == "" {
					t.Error("TestFileHandler() missing Content-Range header for partial content")
				}
			}
		})
	}
}

func TestTestFileHandlerWithCustomHeaders(t *testing.T) {
	req := httptest.NewRequest("GET", "/1k", nil)
	req.Header.Set("X-Cache-Control", "no-cache")
	req.URL.RawQuery = "X-Custom-Header=test-value"
	w := httptest.NewRecorder()

	TestFileHandler(w, req, defaultTestFileCC)

	resp := w.Result()

	if got := resp.Header.Get("Cache-Control"); got != "no-cache" {
		t.Errorf("TestFileHandler() Cache-Control = %v, want no-cache", got)
	}

	if got := resp.Header.Get("X-Custom-Header"); got != "test-value" {
		t.Errorf("TestFileHandler() X-Custom-Header = %v, want test-value", got)
	}
}

func TestReTestFileRegex(t *testing.T) {
	tests := []struct {
		path     string
		expected []string
	}{
		{"/1k", []string{"1k", "1", "k"}},
		{"/500b", []string{"500b", "500", "b"}},
		{"/2m.json", []string{"2m.json", "2", "m", ".json"}},
		{"/10g.txt", []string{"10g.txt", "10", "g", ".txt"}},
		{"/invalid", nil}, // Should not match
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			matches := reTestFile.FindStringSubmatch(tt.path)
			if tt.expected == nil {
				if matches != nil {
					t.Errorf("reTestFile should not match %v, but got %v", tt.path, matches)
				}
			} else {
				if matches == nil {
					t.Errorf("reTestFile should match %v", tt.path)
					return
				}
				for i, expected := range tt.expected {
					if i < len(matches) && matches[i] != expected {
						t.Errorf("reTestFile match %d = %v, want %v", i, matches[i], expected)
					}
				}
			}
		})
	}
}
