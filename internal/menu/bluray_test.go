package menu

import (
	"image"
	"os"
	"path/filepath"
	"testing"

	"easydvd/internal/model"
)

func TestBluRayMenusHaveHDArtworkAndTypedActions(t *testing.T) {
	s := model.DefaultSettings(t.TempDir())
	s.DiscFormat = "Blu-ray"
	s.Media = "BD-25"
	s.TitlesPerPage = 1
	videos := []model.Video{{Label: "First"}, {Label: "Second"}}
	for _, aspect := range []string{"4:3", "16:9"} {
		s.Menu.Aspect = aspect
		dir := filepath.Join(t.TempDir(), "menus")
		pages, err := Render(dir, "Collection", 1, 1, videos, s)
		if err != nil {
			t.Fatal(err)
		}
		if len(pages) != 2 {
			t.Fatal(pages)
		}
		for _, p := range pages {
			f, err := os.Open(p.Background)
			if err != nil {
				t.Fatal(err)
			}
			img, _, err := image.Decode(f)
			f.Close()
			if err != nil {
				t.Fatal(err)
			}
			if img.Bounds().Dx() != 1920 || img.Bounds().Dy() != 1080 {
				t.Fatal(img.Bounds())
			}
			if _, err = os.Stat(p.XML); !os.IsNotExist(err) {
				t.Fatal("Blu-ray generated DVD subpicture XML")
			}
			for _, b := range p.Buttons {
				if !b.Rect.In(img.Bounds()) || b.Action.Kind == "" || b.Command != "" {
					t.Fatal(b)
				}
				if _, err = os.Stat(b.Selected); err != nil {
					t.Fatal(err)
				}
			}
		}
		if pages[0].Buttons[0].Action.Target != 1 || pages[1].Buttons[0].Action.Target != 2 {
			t.Fatal("title destinations lost")
		}
	}
}
