package http

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDirList1(t *testing.T) {
	// Create a temporary directory with various file types
	tmpDir := t.TempDir()

	// Create test files with different extensions
	testFiles := []struct {
		name    string
		content string
		size    int64
	}{
		{"test.txt", "test content", 12},
		{"video.mp4", "fake video data", 15},
		{"image.png", "fake image data", 15},
		{"code.go", "package main\n", 13},
		{"audio.mp3", "fake audio data", 15},
	}

	for _, tf := range testFiles {
		filePath := filepath.Join(tmpDir, tf.name)
		err := os.WriteFile(filePath, []byte(tf.content), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file %s: %v", tf.name, err)
		}
	}

	// Create a subdirectory
	subDir := filepath.Join(tmpDir, "subdir")
	err := os.Mkdir(subDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create subdirectory: %v", err)
	}

	// Open the directory
	f, err := os.Open(tmpDir)
	if err != nil {
		t.Fatalf("Failed to open directory: %v", err)
	}
	defer f.Close()

	// Test the dirList1 function
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)

	dirList1(w, f, req)

	resp := w.Result()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("dirList1() status = %v, want %v", resp.StatusCode, http.StatusOK)
	}

	body := w.Body.String()

	// Check that the response contains HTML
	if !strings.Contains(body, "<html") {
		t.Error("dirList1() response should contain HTML")
	}

	// Check that all test files are listed
	for _, tf := range testFiles {
		if !strings.Contains(body, tf.name) {
			t.Errorf("dirList1() response should contain file %s", tf.name)
		}
	}

	// Check that subdirectory is listed
	if !strings.Contains(body, "subdir/") {
		t.Error("dirList1() response should contain subdirectory with trailing slash")
	}

	// Check for proper CSS and JS includes
	if !strings.Contains(body, "bootstrap.min.css") {
		t.Error("dirList1() response should include Bootstrap CSS")
	}

	if !strings.Contains(body, "font-awesome.min.css") {
		t.Error("dirList1() response should include FontAwesome CSS")
	}
}

func TestDirList1WithEmptyDirectory(t *testing.T) {
	// Create an empty temporary directory
	tmpDir := t.TempDir()

	f, err := os.Open(tmpDir)
	if err != nil {
		t.Fatalf("Failed to open directory: %v", err)
	}
	defer f.Close()

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)

	dirList1(w, f, req)

	resp := w.Result()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("dirList1() status = %v, want %v", resp.StatusCode, http.StatusOK)
	}

	body := w.Body.String()

	// Should still contain HTML structure even with empty directory
	if !strings.Contains(body, "<html") {
		t.Error("dirList1() response should contain HTML even for empty directory")
	}

	// Should contain the table structure
	if !strings.Contains(body, "<table") {
		t.Error("dirList1() response should contain table structure")
	}
}

func TestDirList1FileIcons(t *testing.T) {
	// Test that different file types get appropriate icons
	tmpDir := t.TempDir()

	// Create files with different extensions that should get specific icons
	iconTests := []struct {
		filename     string
		expectedIcon string
	}{
		{"video.mp4", "file-video-o"},
		{"audio.mp3", "file-audio-o"},
		{"image.png", "file-image-o"},
		{"code.go", "file-code-o"},
		{"document.txt", "file-text-o"}, // Default icon
	}

	for _, test := range iconTests {
		filePath := filepath.Join(tmpDir, test.filename)
		err := os.WriteFile(filePath, []byte("test content"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file %s: %v", test.filename, err)
		}
	}

	f, err := os.Open(tmpDir)
	if err != nil {
		t.Fatalf("Failed to open directory: %v", err)
	}
	defer f.Close()

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)

	dirList1(w, f, req)

	body := w.Body.String()

	// Check that different file types get the expected icons
	for _, test := range iconTests {
		if !strings.Contains(body, test.expectedIcon) {
			t.Errorf("dirList1() should contain icon %s for file %s", test.expectedIcon, test.filename)
		}
	}
}

func TestDirList1ParentDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	f, err := os.Open(tmpDir)
	if err != nil {
		t.Fatalf("Failed to open directory: %v", err)
	}
	defer f.Close()

	// Test with a subdirectory request (should show parent directory link)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/subdir/", nil)

	dirList1(w, f, req)

	body := w.Body.String()

	// Should contain parent directory link for subdirectories
	if !strings.Contains(body, "Parent Directory") {
		t.Error("dirList1() should contain Parent Directory link for subdirectories")
	}

	if !strings.Contains(body, "folder-open-o") {
		t.Error("dirList1() should contain folder icon for parent directory")
	}
}

func TestDirList1FileSize(t *testing.T) {
	tmpDir := t.TempDir()

	// Create files with different sizes
	files := []struct {
		name string
		size int
	}{
		{"small.txt", 100},
		{"medium.txt", 1024},       // 1KB
		{"large.txt", 1024 * 1024}, // 1MB
	}

	for _, f := range files {
		content := strings.Repeat("a", f.size)
		filePath := filepath.Join(tmpDir, f.name)
		err := os.WriteFile(filePath, []byte(content), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file %s: %v", f.name, err)
		}
	}

	dir, err := os.Open(tmpDir)
	if err != nil {
		t.Fatalf("Failed to open directory: %v", err)
	}
	defer dir.Close()

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)

	dirList1(w, dir, req)

	body := w.Body.String()

	// Check that file sizes are humanized (using go-humanize)
	// The exact format might vary, but should contain size information
	if !strings.Contains(body, "100 B") && !strings.Contains(body, "100") {
		t.Error("dirList1() should show file size for small.txt")
	}

	if !strings.Contains(body, "1.0 kB") && !strings.Contains(body, "1024") {
		t.Error("dirList1() should show file size for medium.txt")
	}

	if !strings.Contains(body, "1.0 MB") && !strings.Contains(body, "1048576") {
		t.Error("dirList1() should show file size for large.txt")
	}
}
