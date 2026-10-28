package menu

import (
	"crypto/sha256"
	"easydvd/internal/model"
	"golang.org/x/image/font/gofont/gomonobold"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"testing"
)

func TestPreviewUsesStyleAndDoesNotWriteFiles(t *testing.T) {
	dir := t.TempDir()
	s := model.DefaultSettings(dir)
	videos := []model.Video{{Label: "Custom typography"}}
	bg := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			bg.Set(x, y, color.RGBA{180, 40, 20, 255})
		}
	}
	path := filepath.Join(dir, "background.png")
	if err := save(path, bg); err != nil {
		t.Fatal(err)
	}
	fontPath := filepath.Join(dir, "font.ttf")
	if err := os.WriteFile(fontPath, gomonobold.TTF, 0644); err != nil {
		t.Fatal(err)
	}
	s.Menu.Background = path
	s.Menu.Dimming = 0
	s.Menu.Aspect = "16:9"
	first, err := Preview("Family", 1, 1, videos, s, 1)
	if err != nil {
		t.Fatal(err)
	}
	if first.Bounds().Dx() != 1024 || first.At(0, 0) != (color.RGBA{180, 40, 20, 255}) {
		t.Fatal("background/aspect ignored")
	}
	before := sha256.Sum256(first.(*image.RGBA).Pix)
	s.Menu.Font = "Custom"
	s.Menu.FontFile = fontPath
	s.Menu.FontColor = "#00FF00"
	s.Menu.FontSize = 30
	second, err := Preview("Family", 1, 1, videos, s, 1)
	if err != nil {
		t.Fatal(err)
	}
	if sha256.Sum256(second.(*image.RGBA).Pix) == before {
		t.Fatal("font/color settings ignored")
	}
	s.Menu.Dimming = 50
	third, err := Preview("Family", 1, 1, videos, s, 1)
	if err != nil {
		t.Fatal(err)
	}
	if r, _, _, _ := third.At(0, 0).RGBA(); r >= 180*257 {
		t.Fatal("dimming ignored")
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 2 {
		t.Fatal("fast preview wrote artifacts", entries)
	}
	s.Menu.Background = filepath.Join(dir, "missing.png")
	if _, err = Preview("Family", 1, 1, videos, s, 1); err == nil {
		t.Fatal("missing background silently ignored")
	}
}

func BenchmarkPreview(b *testing.B) {
	s := model.DefaultSettings(b.TempDir())
	v := make([]model.Video, 74)
	for i := range v {
		v[i].Label = "Family film with fine detail"
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := Preview("Collection", 1, 3, v, s, 1); err != nil {
			b.Fatal(err)
		}
	}
}
