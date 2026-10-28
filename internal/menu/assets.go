package menu

import (
	"fmt"
	"image"
	"image/color"
	"os"
	"strconv"
	"sync"

	"easydvd/internal/model"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/gomono"
	"golang.org/x/image/font/gofont/gomonobold"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
)

var assets = struct {
	sync.Mutex
	fonts  map[string]*opentype.Font
	images map[string]image.Image
}{fonts: map[string]*opentype.Font{}, images: map[string]image.Image{}}

func Color(s string) color.RGBA {
	v, _ := strconv.ParseUint(s[1:], 16, 32)
	return color.RGBA{uint8(v >> 16), uint8(v >> 8), uint8(v), 255}
}
func fontFile(key string, data []byte) (*opentype.Font, error) {
	assets.Lock()
	defer assets.Unlock()
	if f := assets.fonts[key]; f != nil {
		return f, nil
	}
	if data == nil {
		info, err := os.Stat(key)
		if err != nil {
			return nil, err
		}
		if info.Size() > 20<<20 {
			return nil, fmt.Errorf("font file exceeds 20 MB")
		}
		var e error
		data, e = os.ReadFile(key)
		if e != nil {
			return nil, e
		}
	}
	f, err := opentype.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("invalid TTF/OTF font: %w", err)
	}
	if len(assets.fonts) >= 32 {
		assets.fonts = map[string]*opentype.Font{}
	}
	assets.fonts[key] = f
	return f, nil
}
func typefaces(m model.MenuStyle) (*opentype.Font, *opentype.Font, error) {
	key, data, bold := "builtin-sans", goregular.TTF, gobold.TTF
	if m.Font == "Monospace" {
		key, data, bold = "builtin-mono", gomono.TTF, gomonobold.TTF
	}
	if m.Font == "Custom" {
		f, e := fontFile(m.FontFile, nil)
		return f, f, e
	}
	regular, e := fontFile(key, data)
	if e != nil {
		return nil, nil, e
	}
	b, e := fontFile(key+"-bold", bold)
	return regular, b, e
}
func background(path string) (image.Image, error) {
	assets.Lock()
	defer assets.Unlock()
	if pic := assets.images[path]; pic != nil {
		return pic, nil
	}
	f, e := os.Open(path)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	info, e := f.Stat()
	if e != nil {
		return nil, e
	}
	if info.Size() > 32<<20 {
		return nil, fmt.Errorf("image exceeds 32 MB")
	}
	cfg, _, e := image.DecodeConfig(f)
	if e != nil {
		return nil, e
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || int64(cfg.Width)*int64(cfg.Height) > 24000000 {
		return nil, fmt.Errorf("image must contain at most 24 million pixels")
	}
	if _, e = f.Seek(0, 0); e != nil {
		return nil, e
	}
	pic, _, e := image.Decode(f)
	if e != nil {
		return nil, e
	}
	if len(assets.images) >= 3 {
		assets.images = map[string]image.Image{}
	}
	assets.images[path] = pic
	return pic, nil
}
func ValidateAsset(path, kind string) error {
	if kind == "background" {
		_, e := background(path)
		return e
	}
	_, e := fontFile(path, nil)
	return e
}
