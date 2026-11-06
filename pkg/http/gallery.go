package http

import (
	"bytes"
	"container/list"
	_ "embed"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"path"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "golang.org/x/image/bmp"
	xdraw "golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

//go:embed gallery.html
var galleryHTML []byte

// imageExts are shown in the gallery. Formats Go cannot decode (svg/avif/heic)
// are still listed; their "thumbnail" falls back to the original file.
var imageExts = map[string]bool{
	"jpg": true, "jpeg": true, "png": true, "gif": true, "webp": true,
	"bmp": true, "avif": true, "svg": true, "heic": true, "jfif": true,
}

// thumbable lists formats the server can decode and downscale.
var thumbable = map[string]bool{
	"jpg": true, "jpeg": true, "jfif": true, "png": true, "gif": true, "webp": true, "bmp": true,
}

func extOf(name string) string {
	return strings.ToLower(strings.TrimPrefix(path.Ext(name), "."))
}

// IsImageFile reports whether the gallery treats name as an image.
func IsImageFile(name string) bool { return imageExts[extOf(name)] }

type galleryEntry struct {
	Name  string `json:"name"`
	Size  int64  `json:"size"`
	MTime int64  `json:"mtime"`
	Thumb bool   `json:"thumb"`
}

type galleryListing struct {
	Path   string         `json:"path"`
	Dirs   []galleryEntry `json:"dirs"`
	Images []galleryEntry `json:"images"`
	Others int            `json:"others"`
}

// serveGallery renders the embedded gallery page (?gallery) or its JSON
// listing (?gallery=json) for directory f.
func serveGallery(w http.ResponseWriter, r *http.Request, f http.File) {
	if r.URL.Query().Get("gallery") != "json" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		w.Write(galleryHTML)
		return
	}
	infos, err := f.Readdir(-1)
	if err != nil {
		http.Error(w, "Error reading directory", http.StatusInternalServerError)
		return
	}
	out := galleryListing{Path: r.URL.Path, Dirs: []galleryEntry{}, Images: []galleryEntry{}}
	for _, fi := range infos {
		name := fi.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		e := galleryEntry{Name: name, Size: fi.Size(), MTime: fi.ModTime().Unix()}
		switch {
		case fi.IsDir():
			out.Dirs = append(out.Dirs, e)
		case IsImageFile(name):
			e.Thumb = thumbable[extOf(name)]
			out.Images = append(out.Images, e)
		default:
			out.Others++
		}
	}
	sort.Slice(out.Dirs, func(i, j int) bool { return naturalLess(out.Dirs[i].Name, out.Dirs[j].Name) })
	sort.Slice(out.Images, func(i, j int) bool { return naturalLess(out.Images[i].Name, out.Images[j].Name) })
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	json.NewEncoder(w).Encode(out)
}

// naturalLess compares case-insensitively with digit runs compared numerically
// (img2 < img10), matching how desktop photo browsers order files.
func naturalLess(a, b string) bool {
	a, b = strings.ToLower(a), strings.ToLower(b)
	for a != "" && b != "" {
		if isDigit(a[0]) && isDigit(b[0]) {
			i, j := digitRun(a), digitRun(b)
			na, nb := strings.TrimLeft(a[:i], "0"), strings.TrimLeft(b[:j], "0")
			if len(na) != len(nb) {
				return len(na) < len(nb)
			}
			if na != nb {
				return na < nb
			}
			a, b = a[i:], b[j:]
			continue
		}
		if a[0] != b[0] {
			return a[0] < b[0]
		}
		a, b = a[1:], b[1:]
	}
	return len(a) < len(b)
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }
func digitRun(s string) int {
	i := 0
	for i < len(s) && isDigit(s[i]) {
		i++
	}
	return i
}

// ---- thumbnails ----

const thumbCacheMaxBytes = 128 << 20

type thumbCache struct {
	mu    sync.Mutex
	ll    *list.List
	items map[string]*list.Element
	bytes int
}

type thumbItem struct {
	key  string
	data []byte
}

var (
	thumbs   = &thumbCache{ll: list.New(), items: map[string]*list.Element{}}
	thumbSem = make(chan struct{}, max(2, runtime.NumCPU()/2))
)

func (c *thumbCache) get(k string) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if e, ok := c.items[k]; ok {
		c.ll.MoveToFront(e)
		return e.Value.(*thumbItem).data, true
	}
	return nil, false
}

func (c *thumbCache) put(k string, data []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.items[k]; ok {
		return
	}
	c.items[k] = c.ll.PushFront(&thumbItem{k, data})
	c.bytes += len(data)
	for c.bytes > thumbCacheMaxBytes && c.ll.Len() > 1 {
		e := c.ll.Back()
		it := e.Value.(*thumbItem)
		c.ll.Remove(e)
		delete(c.items, it.key)
		c.bytes -= len(it.data)
	}
}

// serveThumb answers ?thumb=N for an image file with a JPEG whose longest edge
// is at most N pixels. Unsupported formats or decode failures fall through to
// the original file (returns false).
func serveThumb(w http.ResponseWriter, r *http.Request, f http.File, name string, d interface {
	Size() int64
	ModTime() time.Time
}) bool {
	if !thumbable[extOf(name)] {
		return false
	}
	size, _ := strconv.Atoi(r.URL.Query().Get("thumb"))
	if size <= 0 {
		size = 256
	}
	size = min(max(size, 32), 1600)
	etag := fmt.Sprintf(`"t%d-%x-%x"`, size, d.ModTime().UnixNano(), d.Size())
	w.Header().Set("ETag", etag)
	w.Header().Set("Cache-Control", "public, max-age=86400")
	if inm := r.Header.Get("If-None-Match"); inm != "" && etagWeakMatch(inm, etag) {
		w.WriteHeader(http.StatusNotModified)
		return true
	}
	key := name + "|" + etag
	data, ok := thumbs.get(key)
	if !ok {
		thumbSem <- struct{}{}
		data, ok = thumbs.get(key) // another request may have produced it meanwhile
		if !ok {
			var err error
			data, err = makeThumb(f, size)
			if err != nil {
				<-thumbSem
				f.Seek(0, io.SeekStart)
				return false
			}
			thumbs.put(key, data)
		}
		<-thumbSem
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	if r.Method != http.MethodHead {
		w.Write(data)
	}
	return true
}

func makeThumb(f io.ReadSeeker, size int) ([]byte, error) {
	raw, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}
	src, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	orient := jpegOrientation(raw)
	b := src.Bounds()
	sw, sh := b.Dx(), b.Dy()
	if sw == 0 || sh == 0 {
		return nil, fmt.Errorf("empty image")
	}
	tw, th := sw, sh
	if sw > size || sh > size {
		if sw >= sh {
			tw, th = size, max(1, sh*size/sw)
		} else {
			tw, th = max(1, sw*size/sh), size
		}
	}
	dst := image.NewRGBA(image.Rect(0, 0, tw, th))
	// white matte for transparent PNG/GIF since JPEG has no alpha
	xdraw.Draw(dst, dst.Bounds(), image.NewUniform(color.White), image.Point{}, xdraw.Src)
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, b, xdraw.Over, nil)
	out := applyOrientation(dst, orient)
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, out, &jpeg.Options{Quality: 82}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// jpegOrientation extracts the EXIF Orientation tag (1-8) from JPEG bytes, or 1.
func jpegOrientation(b []byte) int {
	if len(b) < 4 || b[0] != 0xFF || b[1] != 0xD8 {
		return 1
	}
	i := 2
	for i+4 <= len(b) {
		if b[i] != 0xFF {
			return 1
		}
		marker := b[i+1]
		segLen := int(binary.BigEndian.Uint16(b[i+2:]))
		if marker == 0xDA || segLen < 2 || i+2+segLen > len(b) {
			return 1
		}
		seg := b[i+4 : i+2+segLen]
		if marker == 0xE1 && len(seg) > 14 && string(seg[:6]) == "Exif\x00\x00" {
			return tiffOrientation(seg[6:])
		}
		i += 2 + segLen
	}
	return 1
}

func tiffOrientation(t []byte) int {
	if len(t) < 8 {
		return 1
	}
	var bo binary.ByteOrder
	switch string(t[:2]) {
	case "II":
		bo = binary.LittleEndian
	case "MM":
		bo = binary.BigEndian
	default:
		return 1
	}
	off := int(bo.Uint32(t[4:]))
	if off+2 > len(t) {
		return 1
	}
	n := int(bo.Uint16(t[off:]))
	for k := 0; k < n; k++ {
		e := off + 2 + k*12
		if e+12 > len(t) {
			return 1
		}
		if bo.Uint16(t[e:]) == 0x0112 {
			v := int(bo.Uint16(t[e+8:]))
			if v >= 1 && v <= 8 {
				return v
			}
			return 1
		}
	}
	return 1
}

// applyOrientation transforms img so it displays upright per EXIF orientation o.
func applyOrientation(img *image.RGBA, o int) image.Image {
	if o <= 1 || o > 8 {
		return img
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	ow, oh := w, h
	if o >= 5 {
		ow, oh = h, w
	}
	out := image.NewRGBA(image.Rect(0, 0, ow, oh))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var nx, ny int
			switch o {
			case 2:
				nx, ny = w-1-x, y
			case 3:
				nx, ny = w-1-x, h-1-y
			case 4:
				nx, ny = x, h-1-y
			case 5:
				nx, ny = y, x
			case 6:
				nx, ny = h-1-y, x
			case 7:
				nx, ny = h-1-y, w-1-x
			case 8:
				nx, ny = y, w-1-x
			}
			out.SetRGBA(nx, ny, img.RGBAAt(x, y))
		}
	}
	return out
}
