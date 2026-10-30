package model

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestSnapshotIsIndependent(t *testing.T) {
	j := Job{Videos: []Video{{Label: "Original"}}, Discs: []Disc{{VideoIDs: []string{"one"}}}}
	mode := true
	j.NoMenuOverride = &mode
	s := j.Clone()
	s.Videos[0].Label = "Changed"
	s.Discs[0].VideoIDs[0] = "two"
	*s.NoMenuOverride = false
	if j.Videos[0].Label != "Original" || j.Discs[0].VideoIDs[0] != "one" || !*j.NoMenuOverride {
		t.Fatal("snapshot shares mutable state")
	}
}

func TestRemovedBackendPreferencesMigrate(t *testing.T) {
	s := DefaultSettings(t.TempDir())
	if err := json.Unmarshal([]byte(`{"video_encoder":"HCenc","hcenc_dir":"old location","encoders":4,"standard":"NTSC"}`), &s); err != nil {
		t.Fatal(err)
	}
	if s.Encoders != 2 || s.Standard != "NTSC" {
		t.Fatal(s)
	}
	if err := s.Validate(); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(s)
	if err != nil || strings.Contains(string(data), "hcenc") || strings.Contains(string(data), "video_encoder") {
		t.Fatal(string(data), err)
	}
	if s.EncodingKey() == Hash("mpeg2-v4-pillarbox-bounds-8000-9000-ac3-256-2pass-FFmpeg-NTSC") {
		t.Fatal("old encoding quality checkpoint was accepted")
	}
}

func TestProfilesPersistAndRetainLegacyQuality(t *testing.T) {
	s := DefaultSettings(t.TempDir())
	if s.Profile() != ProfileBalanced {
		t.Fatal("new jobs must default to Balanced")
	}
	balancedKey := s.EncodingKey()
	balancedEstimate := (Video{Duration: 3600}).EstimatedBytes(s)
	s.EncodingProfile = ProfileQuality
	qualityKey := s.EncodingKey()
	if qualityKey == balancedKey || qualityKey != Hash("mpeg2-v5-quality-ceiling-ac3-448-2pass-PAL") {
		t.Fatal("quality checkpoint identity changed", qualityKey)
	}
	if (Video{Duration: 3600}).EstimatedBytes(s) <= balancedEstimate {
		t.Fatal("profile ignored in space estimate")
	}
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var saved Settings
	if err = json.Unmarshal(data, &saved); err != nil || saved.Profile() != ProfileQuality || saved.EncodingKey() != qualityKey {
		t.Fatal(saved, err)
	}
	var legacy Settings
	if err = json.Unmarshal([]byte(`{"standard":"PAL"}`), &legacy); err != nil || legacy.Profile() != ProfileQuality || legacy.EncodingKey() != qualityKey {
		t.Fatal("legacy job profile changed", legacy, err)
	}
	defaults := DefaultSettings(t.TempDir())
	if err = json.Unmarshal([]byte(`{"standard":"NTSC"}`), &defaults); err != nil || defaults.Profile() != ProfileBalanced {
		t.Fatal("old preferences overrode new default", defaults, err)
	}
}

func TestOriginalVideoLabel(t *testing.T) {
	v := Video{Path: filepath.Join("clips", "Family.trip.1999.mp4"), Label: "Renamed title"}
	if v.OriginalLabel() != "Family.trip.1999" {
		t.Fatal("original title is not derived from the source filename")
	}
	v.Path = ""
	v.Relative = filepath.Join("nested", "Holiday.MOV")
	if v.OriginalLabel() != "Holiday" {
		t.Fatal("relative source path cannot restore the original title")
	}
}
func TestEncodingKeyAndSettings(t *testing.T) {
	s := DefaultSettings(t.TempDir())
	original := s.EncodingKey()
	s.TitlesPerPage = 4
	s.Menu.Aspect = "16:9"
	s.Menu.FontColor = "#FFCC00"
	s.Packing = "Space efficiency"
	s.OutputDir = filepath.Join(t.TempDir(), "output")
	if s.EncodingKey() != original {
		t.Fatal("menu/packing changes invalidate encoded media")
	}
	s.Standard = "NTSC"
	if s.EncodingKey() == original {
		t.Fatal("format change failed to invalidate encoding")
	}
	s.TitlesPerPage = 13
	if s.Validate() == nil {
		t.Fatal("illegal menu density accepted")
	}
}
