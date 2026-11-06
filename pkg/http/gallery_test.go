package http

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestNaturalLess(t *testing.T) {
	cases := [][2]string{{"img2", "img10"}, {"IMG1", "img2"}, {"a", "b"}, {"x9y", "x10a"}, {"a", "ab"}, {"007", "8"}}
	for _, c := range cases {
		if !naturalLess(c[0], c[1]) || naturalLess(c[1], c[0]) {
			t.Errorf("want %q < %q", c[0], c[1])
		}
	}
}

// jpegWithOrientation builds a w×h JPEG whose left half is red, with an EXIF
// APP1 segment carrying the given orientation.
func jpegWithOrientation(t *testing.T, w, h, o int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := color.RGBA{0, 0, 255, 255}
			if x < w/2 {
				c = color.RGBA{255, 0, 0, 255}
			}
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 95}); err != nil {
		t.Fatal(err)
	}
	b := buf.Bytes()
	// TIFF (big endian): header + 1 IFD entry (0x0112 SHORT 1 = o)
	tiff := []byte{'M', 'M', 0, 42, 0, 0, 0, 8, 0, 1, 0x01, 0x12, 0, 3, 0, 0, 0, 1, 0, byte(o), 0, 0, 0, 0, 0, 0}
	payload := append([]byte("Exif\x00\x00"), tiff...)
	seg := []byte{0xFF, 0xE1, byte((len(payload) + 2) >> 8), byte(len(payload) + 2)}
	seg = append(seg, payload...)
	return append(append([]byte{0xFF, 0xD8}, seg...), b[2:]...)
}

func TestJpegOrientation(t *testing.T) {
	for _, o := range []int{1, 3, 6, 8} {
		if got := jpegOrientation(jpegWithOrientation(t, 8, 4, o)); got != o {
			t.Errorf("orientation %d parsed as %d", o, got)
		}
	}
	if jpegOrientation([]byte("nope")) != 1 {
		t.Error("garbage should be orientation 1")
	}
}

func galleryServer(t *testing.T, noindex bool) (*httptest.Server, string) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "img10.jpg"), jpegWithOrientation(t, 400, 200, 1), 0644)
	os.WriteFile(filepath.Join(dir, "img2.jpg"), jpegWithOrientation(t, 400, 200, 6), 0644)
	os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("x"), 0644)
	os.WriteFile(filepath.Join(dir, ".hidden.png"), []byte("x"), 0644)
	os.Mkdir(filepath.Join(dir, "sub"), 0755)
	return httptest.NewServer(FileServerWithOptions(Dir(dir), noindex)), dir
}

func TestGalleryJSONAndPage(t *testing.T) {
	ts, _ := galleryServer(t, false)
	defer ts.Close()
	res, err := http.Get(ts.URL + "/?gallery=json")
	if err != nil {
		t.Fatal(err)
	}
	var l galleryListing
	json.NewDecoder(res.Body).Decode(&l)
	res.Body.Close()
	if len(l.Images) != 2 || l.Images[0].Name != "img2.jpg" || l.Images[1].Name != "img10.jpg" {
		t.Fatalf("images = %+v", l.Images)
	}
	if len(l.Dirs) != 1 || l.Dirs[0].Name != "sub" || l.Others != 1 {
		t.Fatalf("dirs=%+v others=%d", l.Dirs, l.Others)
	}
	res, _ = http.Get(ts.URL + "/?gallery")
	body := new(bytes.Buffer)
	body.ReadFrom(res.Body)
	res.Body.Close()
	if res.Header.Get("Content-Type") != "text/html; charset=utf-8" || !bytes.Contains(body.Bytes(), []byte(`id="viewer"`)) {
		t.Fatal("gallery page not served")
	}
	res, _ = http.Get(ts.URL + "/")
	body.Reset()
	body.ReadFrom(res.Body)
	res.Body.Close()
	if !bytes.Contains(body.Bytes(), []byte("Gallery (2)")) {
		t.Fatal("listing lacks Gallery link")
	}
}

func TestThumbOrientationAndETag(t *testing.T) {
	ts, _ := galleryServer(t, false)
	defer ts.Close()
	res, err := http.Get(ts.URL + "/img2.jpg?thumb=100")
	if err != nil {
		t.Fatal(err)
	}
	img, err := jpeg.Decode(res.Body)
	res.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	// 400x200 stored with orientation 6 -> displayed 200x400, scaled to 50x100
	if b := img.Bounds(); b.Dx() != 50 || b.Dy() != 100 {
		t.Fatalf("thumb size %v", b)
	}
	// red (left half of stored image) must end up on top after rotating 90° CW
	if r, _, bl, _ := img.At(25, 10).RGBA(); r>>8 < 200 || bl>>8 > 60 {
		t.Fatalf("orientation not applied: top pixel r=%d b=%d", r>>8, bl>>8)
	}
	req, _ := http.NewRequest("GET", ts.URL+"/img2.jpg?thumb=100", nil)
	req.Header.Set("If-None-Match", res.Header.Get("ETag"))
	res2, _ := http.DefaultClient.Do(req)
	res2.Body.Close()
	if res2.StatusCode != http.StatusNotModified {
		t.Fatalf("conditional thumb = %d", res2.StatusCode)
	}
}

func TestGalleryRespectsNoindex(t *testing.T) {
	ts, _ := galleryServer(t, true)
	defer ts.Close()
	res, _ := http.Get(ts.URL + "/?gallery=json")
	res.Body.Close()
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("noindex gallery = %d", res.StatusCode)
	}
}
