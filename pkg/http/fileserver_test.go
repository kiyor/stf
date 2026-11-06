package http

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDirOpen(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	content := "Hello, World!"

	err := os.WriteFile(testFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	dir := Dir(tmpDir)

	tests := []struct {
		name     string
		filename string
		wantErr  bool
	}{
		{
			name:     "valid file",
			filename: "test.txt",
			wantErr:  false,
		},
		{
			name:     "non-existent file",
			filename: "nonexistent.txt",
			wantErr:  true,
		},
		{
			name:     "invalid path with separator",
			filename: string(filepath.Separator) + "test.txt",
			wantErr:  true,
		},
		{
			name:     "path with null byte",
			filename: "test\x00.txt",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "invalid path with separator" && filepath.Separator == '/' {
				t.Skip("separator check only applies where the OS separator is not '/'")
			}
			f, err := dir.Open(tt.filename)
			if (err != nil) != tt.wantErr {
				t.Errorf("Dir.Open() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if f != nil {
				f.Close()
			}
		})
	}
}

func TestFileServer(t *testing.T) {
	// Create a temporary directory structure for testing
	tmpDir := t.TempDir()

	// Create test files
	testFile := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(testFile, []byte("test content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create subdirectory
	subDir := filepath.Join(tmpDir, "subdir")
	err = os.Mkdir(subDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create subdirectory: %v", err)
	}

	subFile := filepath.Join(subDir, "subfile.txt")
	err = os.WriteFile(subFile, []byte("sub content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create sub file: %v", err)
	}

	// Create index.html for directory listing test
	indexFile := filepath.Join(tmpDir, "index.html")
	err = os.WriteFile(indexFile, []byte("<html><body>Index</body></html>"), 0644)
	if err != nil {
		t.Fatalf("Failed to create index file: %v", err)
	}

	fs := FileServer(Dir(tmpDir))
	server := httptest.NewServer(fs)
	defer server.Close()

	tests := []struct {
		name         string
		path         string
		wantStatus   int
		wantContains string
		wantRedirect bool
	}{
		{
			name:         "serve file",
			path:         "/test.txt",
			wantStatus:   http.StatusOK,
			wantContains: "test content",
		},
		{
			name:         "serve subdirectory file",
			path:         "/subdir/subfile.txt",
			wantStatus:   http.StatusOK,
			wantContains: "sub content",
		},
		{
			name:       "directory without trailing slash redirects",
			path:       "/subdir",
			wantStatus: http.StatusMovedPermanently,
		},
		{
			name:         "directory with trailing slash shows index",
			path:         "/",
			wantStatus:   http.StatusOK,
			wantContains: "Index", // Should serve index.html
		},
		{
			name:       "non-existent file",
			path:       "/nonexistent.txt",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "path traversal attempt",
			path:       "/../../../etc/passwd",
			wantStatus: http.StatusNotFound, // cleaned into the root, never escapes it
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Don't follow redirects, and send the path verbatim (Go's client
			// would otherwise clean "/../" before it reaches the server).
			client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			}}
			req, err := http.NewRequest("GET", server.URL, nil)
			if err != nil {
				t.Fatal(err)
			}
			req.URL.Opaque = tt.path
			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("Failed to make request: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tt.wantStatus {
				t.Errorf("Status = %v, want %v", resp.StatusCode, tt.wantStatus)
			}

			if tt.wantContains != "" {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					t.Fatalf("Failed to read response body: %v", err)
				}
				if !strings.Contains(string(body), tt.wantContains) {
					t.Errorf("Response body does not contain %q. Got: %s", tt.wantContains, string(body))
				}
			}
		})
	}
}

func TestServeContent(t *testing.T) {
	content := "Hello, World!"
	modTime := time.Now().Add(-time.Hour) // 1 hour ago

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test.txt", nil)

	ServeContent(w, req, "test.txt", modTime, strings.NewReader(content))

	resp := w.Result()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Status = %v, want %v", resp.StatusCode, http.StatusOK)
	}

	if ct := resp.Header.Get("Content-Type"); ct != "text/plain; charset=utf-8" {
		t.Errorf("Content-Type = %v, want text/plain; charset=utf-8", ct)
	}

	if lm := resp.Header.Get("Last-Modified"); lm == "" {
		t.Error("Last-Modified header is empty")
	}

	body := w.Body.String()
	if body != content {
		t.Errorf("Body = %v, want %v", body, content)
	}
}

func TestServeContentWithRange(t *testing.T) {
	content := "0123456789abcdef"

	tests := []struct {
		name        string
		rangeHeader string
		wantStatus  int
		wantContent string
		wantRange   string
	}{
		{
			name:        "bytes 0-4",
			rangeHeader: "bytes=0-4",
			wantStatus:  http.StatusPartialContent,
			wantContent: "01234",
			wantRange:   "bytes 0-4/16",
		},
		{
			name:        "bytes 5-",
			rangeHeader: "bytes=5-",
			wantStatus:  http.StatusPartialContent,
			wantContent: "56789abcdef",
			wantRange:   "bytes 5-15/16",
		},
		{
			name:        "bytes -5 (last 5 bytes)",
			rangeHeader: "bytes=-5",
			wantStatus:  http.StatusPartialContent,
			wantContent: "bcdef",
			wantRange:   "bytes 11-15/16",
		},
		{
			name:        "invalid range",
			rangeHeader: "bytes=20-30",
			wantStatus:  http.StatusRequestedRangeNotSatisfiable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/test.txt", nil)
			req.Header.Set("Range", tt.rangeHeader)

			ServeContent(w, req, "test.txt", time.Time{}, strings.NewReader(content))

			resp := w.Result()

			if resp.StatusCode != tt.wantStatus {
				t.Errorf("Status = %v, want %v", resp.StatusCode, tt.wantStatus)
			}

			if tt.wantStatus == http.StatusPartialContent {
				body := w.Body.String()
				if body != tt.wantContent {
					t.Errorf("Body = %q, want %q", body, tt.wantContent)
				}

				if cr := resp.Header.Get("Content-Range"); cr != tt.wantRange {
					t.Errorf("Content-Range = %q, want %q", cr, tt.wantRange)
				}
			}
		})
	}
}

func TestCheckPreconditions(t *testing.T) {
	modTime := time.Now().Add(-time.Hour)

	tests := []struct {
		name       string
		headers    map[string]string
		wantDone   bool
		wantStatus int
	}{
		{
			name:     "no preconditions",
			headers:  map[string]string{},
			wantDone: false,
		},
		{
			name:     "If-Modified-Since with old date",
			headers:  map[string]string{"If-Modified-Since": modTime.Add(-time.Hour).UTC().Format(http.TimeFormat)},
			wantDone: false,
		},
		{
			name:       "If-Modified-Since with recent date",
			headers:    map[string]string{"If-Modified-Since": modTime.Add(time.Hour).UTC().Format(http.TimeFormat)},
			wantDone:   true,
			wantStatus: http.StatusNotModified,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/test.txt", nil)

			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			done, _ := checkPreconditions(w, req, modTime)

			if done != tt.wantDone {
				t.Errorf("checkPreconditions() done = %v, want %v", done, tt.wantDone)
			}

			if tt.wantDone && tt.wantStatus != 0 {
				resp := w.Result()
				if resp.StatusCode != tt.wantStatus {
					t.Errorf("Status = %v, want %v", resp.StatusCode, tt.wantStatus)
				}
			}
		})
	}
}

func TestContainsDotDot(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"/normal/path", false},
		{"/path/with/../dotdot", true},
		{"/path/with/..dotdot", false}, // Not a directory traversal
		{"/path/with/dotdot..", false}, // Not a directory traversal
		{"../relative", true},
		{"/path\\..\\windows", true}, // Windows-style path separator
		{"/path/..", true},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			if got := containsDotDot(tt.path); got != tt.want {
				t.Errorf("containsDotDot(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestToHTTPError(t *testing.T) {
	tests := []struct {
		name           string
		err            error
		wantMsg        string
		wantStatusCode int
	}{
		{
			name:           "file not found",
			err:            os.ErrNotExist,
			wantMsg:        "404 page not found",
			wantStatusCode: http.StatusNotFound,
		},
		{
			name:           "permission denied",
			err:            os.ErrPermission,
			wantMsg:        "403 Forbidden",
			wantStatusCode: http.StatusForbidden,
		},
		{
			name:           "other error",
			err:            io.EOF,
			wantMsg:        "500 Internal Server Error",
			wantStatusCode: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, code := toHTTPError(tt.err)
			if msg != tt.wantMsg {
				t.Errorf("toHTTPError() msg = %v, want %v", msg, tt.wantMsg)
			}
			if code != tt.wantStatusCode {
				t.Errorf("toHTTPError() code = %v, want %v", code, tt.wantStatusCode)
			}
		})
	}
}
