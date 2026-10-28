package menu

import (
	"easydvd/internal/model"
	"encoding/xml"
	"image"
	"os"
	"path/filepath"
	"testing"
)

func TestPaginationAndBounds(t *testing.T) {
	for _, standard := range []string{"PAL", "NTSC"} {
		for _, aspect := range []string{"4:3", "16:9"} {
			for _, layout := range []string{"Text", "Thumbnails"} {
				dir := t.TempDir()
				s := model.DefaultSettings(dir)
				s.Standard = standard
				s.Menu.Aspect = aspect
				s.Menu.FontSize = 36
				s.Menu.LineSpacing = 30
				s.Layout = layout
				s.TitlesPerPage = 12
				if layout == "Thumbnails" {
					s.TitlesPerPage = 6
				}
				videos := make([]model.Video, 27)
				for i := range videos {
					videos[i].Label = "Film & family <1964> with a long title"
				}
				pages, err := Render(dir, "Archive & family", 1, 3, videos, s)
				if err != nil {
					t.Fatal(err)
				}
				if len(pages) != (27+s.TitlesPerPage-1)/s.TitlesPerPage {
					t.Fatal(len(pages))
				}
				for _, page := range pages {
					for _, b := range page.Buttons {
						if !b.Rect.In(image.Rect(0, 0, 720, s.Height())) || b.Rect.Min.Y%2 != 0 {
							t.Fatal(b)
						}
					}
					for i, a := range page.Buttons {
						for _, b := range page.Buttons[i+1:] {
							if !a.Rect.Intersect(b.Rect).Empty() {
								t.Fatalf("overlapping menu buttons %s and %s (%s %s)", a.Name, b.Name, standard, layout)
							}
						}
					}
					f, err := os.Open(page.XML)
					if err != nil {
						t.Fatal(err)
					}
					var doc any
					if err = xml.NewDecoder(f).Decode(&doc); err != nil {
						t.Fatal(err)
					}
					f.Close()
					f, err = os.Open(page.Preview)
					if err != nil {
						t.Fatal(err)
					}
					cfg, _, err := image.DecodeConfig(f)
					f.Close()
					width := 768
					if aspect == "16:9" {
						width = 1024
					}
					if err != nil || cfg.Width != width || cfg.Height != 576 {
						t.Fatal(cfg, err)
					}
					if _, err = os.Stat(filepath.Join(dir, filepath.Base(page.Background))); err != nil {
						t.Fatal(err)
					}
				}
			}
		}
	}
}
