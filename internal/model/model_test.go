package model

import (
	"path/filepath"
	"testing"
)

func TestSnapshotIsIndependent(t *testing.T) {
	j := Job{Videos: []Video{{Label: "Original"}}, Discs: []Disc{{VideoIDs: []string{"one"}}}}
	s := j.Clone()
	s.Videos[0].Label = "Changed"
	s.Discs[0].VideoIDs[0] = "two"
	if j.Videos[0].Label != "Original" || j.Discs[0].VideoIDs[0] != "one" {
		t.Fatal("snapshot shares mutable state")
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

func TestWorkerLimitsFollowEncoder(t *testing.T) {
	s := DefaultSettings(t.TempDir())
	s.Encoders = 4
	if err := s.Validate(); err != nil {
		t.Fatal(err)
	}
	s.VideoEncoder = "FFmpeg"
	if err := s.Validate(); err == nil {
		t.Fatal("four FFmpeg workers should not be allowed")
	}
	s.Encoders = 2
	if err := s.Validate(); err != nil {
		t.Fatal(err)
	}
}
