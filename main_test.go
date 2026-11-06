package main

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestUploadTargetConfined(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	realRoot, _ := filepath.EvalSymlinks(root)

	cases := []struct {
		in      string
		want    string // relative to root; "" = expect error
		wantErr bool
	}{
		{"/a.txt", "a.txt", false},
		{"/", "index.html", false},
		{"/../../etc/x", "etc/x", false}, // clamped at root, never above
		{"/sub/../b.txt", "b.txt", false},
		{"/d/e/f.txt", "d/e/f.txt", false},
		{"/dir/", "", true},
		{"/link/evil.txt", "", true}, // symlinked parent escapes root
	}
	for _, c := range cases {
		got, err := uploadTarget(root, c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("%q: want error, got %q", c.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("%q: %v", c.in, err)
			continue
		}
		if want := filepath.Join(realRoot, c.want); got != want {
			t.Errorf("%q: got %q want %q", c.in, got, want)
		}
	}
}

func TestUploadHandlerEndToEnd(t *testing.T) {
	root := t.TempDir()
	parent := filepath.Dir(root)
	old := *fdir
	*fdir = root
	defer func() { *fdir = old }()

	// traversal attempt lands inside root
	req := httptest.NewRequest("PUT", "/x", strings.NewReader("pwned"))
	req.URL.Path = "/../escaped-stf-test.txt"
	rec := httptest.NewRecorder()
	uploadHandler(rec, req)
	if rec.Code != 200 {
		t.Fatalf("code %d: %s", rec.Code, rec.Body)
	}
	if _, err := os.Stat(filepath.Join(parent, "escaped-stf-test.txt")); err == nil {
		t.Fatal("file escaped served directory")
	}
	if b, _ := os.ReadFile(filepath.Join(root, "escaped-stf-test.txt")); string(b) != "pwned" {
		t.Fatalf("content %q", b)
	}

	// failing body leaves neither target nor temp file
	req = httptest.NewRequest("PUT", "/broken.bin", errReader{})
	rec = httptest.NewRecorder()
	uploadHandler(rec, req)
	if rec.Code != 500 {
		t.Fatalf("want 500, got %d", rec.Code)
	}
	ents, _ := os.ReadDir(root)
	for _, e := range ents {
		if strings.Contains(e.Name(), "broken") {
			t.Fatalf("leftover %s", e.Name())
		}
	}
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, os.ErrDeadlineExceeded }

func TestParseRedirect(t *testing.T) {
	cases := []struct {
		in   string
		code int
		url  string
	}{
		{"301 https://h/{{.URL.Path}}", 301, "https://h/{{.URL.Path}}"},
		{"301https://h", 301, "https://h"},
		{"https://h", 302, "https://h"},
		{"/", 302, "/"},
		{"307", 307, ""},
		{"200 https://h", 302, "200 https://h"},
	}
	for _, c := range cases {
		code, url := parseRedirect(c.in)
		if code != c.code || url != c.url {
			t.Errorf("%q: got (%d,%q) want (%d,%q)", c.in, code, url, c.code, c.url)
		}
	}
	rec := httptest.NewRecorder()
	handleRedirect(rec, httptest.NewRequest("GET", "/p", nil), "301https://h{{.URL.Path}}")
	if rec.Code != 301 || rec.Header().Get("Location") != "https://h/p" {
		t.Fatalf("got %d %q", rec.Code, rec.Header().Get("Location"))
	}
}

func TestLifecycleIdleShutdown(t *testing.T) {
	l := newLifecycle()
	l.startIdleTimer(50 * time.Millisecond)
	if !l.begin() {
		t.Fatal("begin refused")
	}
	// A request running longer than the timeout must not trigger shutdown.
	time.Sleep(150 * time.Millisecond)
	select {
	case <-l.Done():
		t.Fatal("shut down while a request was active")
	default:
	}
	l.end()
	select {
	case <-l.Done():
	case <-time.After(time.Second):
		t.Fatal("did not shut down after idle timeout")
	}
	if l.begin() {
		t.Fatal("begin accepted after shutdown")
	}
}

func TestLifecycleConcurrent(t *testing.T) {
	l := newLifecycle()
	l.startIdleTimer(time.Millisecond)
	done := make(chan struct{})
	for i := 0; i < 8; i++ {
		go func() {
			for {
				select {
				case <-done:
					return
				default:
				}
				if l.begin() {
					l.end()
				}
			}
		}()
	}
	time.Sleep(50 * time.Millisecond)
	close(done)
	select {
	case <-l.Done():
	case <-time.After(time.Second):
		t.Fatal("never shut down")
	}
	if n := l.activeCount(); n != 0 {
		t.Fatalf("active=%d", n)
	}
}
