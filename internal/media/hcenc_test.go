package media

import (
	"easydvd/internal/model"
	"strings"
	"testing"
)

func TestHCencDVDSettings(t *testing.T) {
	s := model.DefaultSettings(t.TempDir())
	v := model.Video{DAR: 16.0 / 9, FPS: 25, FieldOrder: "bb"}
	ini := HCencINI(v, s)
	for _, want := range []string{"*PROFILE BEST", "*BITRATE 8000", "*MAXBITRATE 9000", "*AUTOGOP 15", "*CLOSEDGOPS", "*ASPECT 16:9", "*INTERLACED\n*BFF", "*COLOUR 5", "*SILENT", "*WAIT 0"} {
		if !strings.Contains(ini, want) {
			t.Fatal(want, ini)
		}
	}
	s.Standard = "NTSC"
	ini = HCencINI(v, s)
	if !strings.Contains(ini, "*AUTOGOP 18") || !strings.Contains(ini, "*PROGRESSIVE\n*PROG_SEQ") || strings.Contains(ini, "*INTERLACED") || !strings.Contains(ini, "*COLOUR 6") {
		t.Fatal(ini)
	}
	for _, forbidden := range []string{"*CQ", "*1PASS", "*LOSSLESS"} {
		if strings.Contains(ini, forbidden) {
			t.Fatal(ini)
		}
	}
}

func TestHCencMustCompleteBothPassesAndEveryFrame(t *testing.T) {
	valid := "encoding - pass 1\nencoding - pass 2\nnr. of frames:           50\r\n"
	if err := validateHCencLog(valid, 50); err != nil {
		t.Fatal(err)
	}
	for _, log := range []string{"", strings.ReplaceAll(valid, "pass 2", "pass failed"), strings.ReplaceAll(valid, "50", "49")} {
		if err := validateHCencLog(log, 50); err == nil {
			t.Fatal("incomplete encode was accepted", log)
		}
	}
	s := model.DefaultSettings(t.TempDir())
	if s.Encoder() != "HCenc" {
		t.Fatal("new jobs must default to HCenc")
	}
	key := s.EncodingKey()
	s.VideoEncoder = "FFmpeg"
	if s.EncodingKey() == key {
		t.Fatal("encoder changes must invalidate cache")
	}
	s.VideoEncoder = ""
	if s.Encoder() != "FFmpeg" || s.EncodingKey() != model.Hash("mpeg2-v2-8000-9000-ac3-256-2pass-PAL") {
		t.Fatal("legacy jobs lost their encoding profile")
	}
}
