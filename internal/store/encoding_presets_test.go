package store

import (
	"easydvd/internal/model"
	"path/filepath"
	"testing"
)

func TestEncodingPresetsPersistReplaceAndDoNotContainWorkers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "queue.db")
	st, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	p := model.EncodingPreset{Name: "Family", DiscFormat: "Blu-ray", Standard: "PAL", Media: "BD-50", EncodingProfile: model.ProfileQuality}
	if err = st.SaveEncodingPreset(p); err != nil {
		t.Fatal(err)
	}
	p.Name = "family"
	p.Media = "BD-25"
	if err = st.SaveEncodingPreset(p); err != nil {
		t.Fatal(err)
	}
	st.Close()
	st, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	got, err := st.EncodingPresets()
	if err != nil || len(got) != 1 || got[0] != p {
		t.Fatal(got, err)
	}
	s := model.DefaultSettings(t.TempDir())
	s.Encoders = 7
	p.Apply(&s)
	if s.Encoders != 7 || !s.IsBluray() || s.Media != "BD-25" {
		t.Fatal(s)
	}
	if err = st.DeleteEncodingPreset("FAMILY"); err != nil {
		t.Fatal(err)
	}
	got, err = st.EncodingPresets()
	if err != nil || len(got) != 0 {
		t.Fatal(got, err)
	}
}

func TestLegacyPreferencesMigrateAppWorkersAndStrictOrder(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "queue.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if _, err = st.db.Exec(`INSERT INTO preferences(key,payload) VALUES('settings',?)`, []byte(`{"encoders":2,"packing":"Space efficiency"}`)); err != nil {
		t.Fatal(err)
	}
	s, err := st.Settings(model.DefaultSettings(t.TempDir()))
	if err != nil || s.Encoders != 0 || !s.AutoCheckUpdates || s.Packing != "Strict order" {
		t.Fatal(s, err)
	}
	s.Encoders = 6
	s.AutoCheckUpdates = false
	if err = st.SaveSettings(s); err != nil {
		t.Fatal(err)
	}
	s, err = st.Settings(model.DefaultSettings(t.TempDir()))
	if err != nil || s.Encoders != 6 || s.AutoCheckUpdates {
		t.Fatal("explicit application preferences lost", s, err)
	}
}
