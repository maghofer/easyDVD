package integration

import (
	"context"
	"easydvd/internal/engine"
	"easydvd/internal/media"
	"easydvd/internal/menu"
	"easydvd/internal/model"
	"easydvd/internal/store"
	"golang.org/x/image/font/gofont/gomono"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestMenuAssetImportAndPresetRecovery(t *testing.T) {
	if os.Getenv("EASYDVD_INTEGRATION") != "1" {
		t.Skip("filesystem helper integration")
	}
	dir := t.TempDir()
	s := model.DefaultSettings(dir)
	st, e := store.Open(filepath.Join(dir, "queue.db"))
	if e != nil {
		t.Fatal(e)
	}
	defer st.Close()
	engine1, e := engine.New(st, s, media.Tools{})
	if e != nil {
		t.Fatal(e)
	}
	defer engine1.Close()
	bg := filepath.Join(dir, "photo.png")
	f, e := os.Create(bg)
	if e != nil {
		t.Fatal(e)
	}
	pic := image.NewRGBA(image.Rect(0, 0, 64, 64))
	pic.Set(0, 0, color.White)
	png.Encode(f, pic)
	f.Close()
	font := filepath.Join(dir, "custom.ttf")
	os.WriteFile(font, gomono.TTF, 0644)
	style := model.DefaultMenuStyle()
	style.Aspect = "16:9"
	style.Font = "Custom"
	style.Background, e = engine1.ImportMenuAsset(context.Background(), bg, "background")
	if e != nil {
		t.Fatal(e)
	}
	style.FontFile, e = engine1.ImportMenuAsset(context.Background(), font, "font")
	if e != nil {
		t.Fatal(e)
	}
	p := model.MenuPreset{Name: "Film archive", Style: style, Layout: "Text", TitlesPerPage: 8}
	if e = engine1.SaveMenuPreset(p); e != nil {
		t.Fatal(e)
	}
	if e = engine1.SetMenu("", style, "Text", 8); e != nil {
		t.Fatal(e)
	}
	os.Remove(bg)
	os.Remove(font)
	engine1.Close()
	engine2, e := engine.New(st, s, media.Tools{})
	if e != nil {
		t.Fatal(e)
	}
	defer engine2.Close()
	presets, e := engine2.MenuPresets()
	if e != nil || len(presets) != 1 || presets[0] != p {
		t.Fatal(presets, e)
	}
	if engine2.Snapshot().Settings.Menu != style {
		t.Fatal("menu defaults did not survive restart")
	}
	if _, e = menu.Preview("Archive", 1, 1, []model.Video{{Label: "Saved asset"}}, engine2.Snapshot().Settings, 1); e != nil {
		t.Fatal(e)
	}
}
