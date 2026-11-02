package model

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBluRaySettingsAndCheckpointSeparation(t *testing.T) {
	s := DefaultSettings(t.TempDir())
	dvdKey := s.EncodingKey()
	b, _ := json.Marshal(s)
	var old Settings
	if err := json.Unmarshal(b, &old); err != nil || old.Format() != "DVD" || old.EncodingKey() != dvdKey {
		t.Fatal(old, err)
	}
	s.DiscFormat = "Blu-ray"
	s.Media = "BD-25"
	if err := s.Validate(); err != nil {
		t.Fatal(err)
	}
	if s.Capacity() != 25000000000 || s.EncodingKey() == dvdKey {
		t.Fatal(s)
	}
	v := Video{Path: "source", Bluray: BlurayPlan{Version: 1}}
	key := v.ArtifactKey(s)
	s.Media = "BD-50"
	s.Menu.FontSize++
	if s.Capacity() != 50000000000 || key != v.ArtifactKey(s) {
		t.Fatal("disc/menu settings invalidated media")
	}
	v.Bluray.CopyVideo = true
	if key == v.ArtifactKey(s) {
		t.Fatal("stream decision missing from checkpoint identity")
	}
	s.DiscFormat = "DVD"
	if s.Validate() == nil {
		t.Fatal("DVD accepted BD capacity")
	}
}

func TestLegacyDVDVideoIdentityOmitsNewMetadata(t *testing.T) {
	s := DefaultSettings(t.TempDir())
	v := Video{Path: "legacy.mp4", Size: 1234, ModTime: 5678}
	key := v.ArtifactKey(s)
	v.Source = StreamMetadata{Codec: "h264", PixelFormat: "yuv420p"}
	if key != v.ArtifactKey(s) {
		t.Fatal("new descriptive fields invalidated DVD encoding")
	}
	v.Source = StreamMetadata{}
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "source_metadata") || strings.Contains(string(b), "bluray_plan") {
		t.Fatal("legacy DVD generation JSON gained new fields", string(b))
	}
}

func TestLegacySettingsOmitNewAppFields(t *testing.T) {
	var s Settings
	if err := json.Unmarshal([]byte(`{"standard":"PAL","media":"DVD-5","packing":"Strict order","encoders":2}`), &s); err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "auto_check_updates") || strings.Contains(string(b), "app_settings_version") {
		t.Fatal("legacy authoring identity gained application preferences", string(b))
	}
}

func TestBlurayAutomaticWorkersFollowProfileAndKeepFixedChoice(t *testing.T) {
	s := DefaultSettings(t.TempDir())
	s.DiscFormat = "Blu-ray"
	if s.EncoderWorkers(16) != 4 || s.EncoderWorkers(1) != 1 {
		t.Fatal("Balanced HD Auto budget", s.EncoderWorkers(16))
	}
	s.EncodingProfile = ProfileQuality
	if s.EncoderWorkers(16) != 2 || s.EncoderWorkers(128) != 4 {
		t.Fatal("Quality HD Auto budget", s.EncoderWorkers(16))
	}
	s.Encoders = 3
	if s.EncoderWorkers(16) != 3 {
		t.Fatal("explicit worker choice changed")
	}
}
