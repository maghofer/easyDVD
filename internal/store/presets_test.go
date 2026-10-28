package store

import (
	"easydvd/internal/model"
	"path/filepath"
	"testing"
)

func TestMenuPresetsPersistAndReplaceCaseInsensitively(t *testing.T) {
	path := filepath.Join(t.TempDir(), "queue.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	p := model.MenuPreset{Name: "Archive", Style: model.DefaultMenuStyle(), Layout: "Text", TitlesPerPage: 10}
	if err = s.SaveMenuPreset(p); err != nil {
		t.Fatal(err)
	}
	p.Name = "archive"
	p.Style.Aspect = "16:9"
	p.Style.Font = "Monospace"
	if err = s.SaveMenuPreset(p); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	got, err := s.MenuPresets()
	if err != nil || len(got) != 1 || got[0] != p {
		t.Fatal("preset round trip failed", got, err)
	}
	p.Style.FontColor = "invalid"
	if s.SaveMenuPreset(p) == nil {
		t.Fatal("invalid preset replaced valid data")
	}
	got, _ = s.MenuPresets()
	if got[0].Style.Aspect != "16:9" {
		t.Fatal("invalid preset damaged saved data")
	}
}
