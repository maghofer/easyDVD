package media

import (
	"easydvd/internal/menu"
	"easydvd/internal/model"
	"encoding/xml"
	"path/filepath"
	"strings"
	"testing"
)

func TestAspectAndInterlace(t *testing.T) {
	s := model.DefaultSettings(t.TempDir())
	v := model.Video{DAR: 4.0 / 3, Width: 1440, Height: 1080, FPS: 25, FieldOrder: "progressive"}
	f, keep := VideoFilter(v, s)
	if keep || !strings.Contains(f, "scale=720:576:") || strings.Contains(f, "bwdif") {
		t.Fatal(f)
	}
	v.DAR = 2.35
	f, _ = VideoFilter(v, s)
	if !strings.Contains(f, "pad=720:576") || strings.Contains(f, "crop") {
		t.Fatal(f)
	}
	v.FieldOrder = "bb"
	_, keep = VideoFilter(v, s)
	if !keep {
		t.Fatal("matching interlacing lost")
	}
	s.Standard = "NTSC"
	f, keep = VideoFilter(v, s)
	if keep || !strings.Contains(f, "bwdif") || !strings.Contains(f, "fps=30000/1001") {
		t.Fatal(f)
	}
}
func TestAudioAndDVDArgs(t *testing.T) {
	s := model.DefaultSettings(t.TempDir())
	v := model.Video{DAR: 4.0 / 3, Width: 720, Height: 576, Duration: 10, VideoStream: 2, AudioStream: -1}
	args := strings.Join(EncodeArgs(v, s, "source", "pass", "video.mpg", 2), " ")
	for _, want := range []string{"-pass 2", "-map 0:2", "anullsrc", "-map 1:a:0", "-af apad", "-b:v 8000k", "-maxrate:v 9000k", "-b:a 256k", "-f dvd", "-g 15"} {
		if !strings.Contains(args, want) {
			t.Fatal(want, args)
		}
	}
	v.AudioStream = 4
	args = strings.Join(EncodeArgs(v, s, "source", "pass", "NUL", 1), " ")
	if strings.Contains(args, "anullsrc") || !strings.Contains(args, "-an -f null") {
		t.Fatal(args)
	}
}
func TestAuthorNavigation(t *testing.T) {
	s := model.DefaultSettings(t.TempDir())
	s.TitlesPerPage = 1
	v := []model.Video{{Encoded: filepath.Join("work", "a & b.mpg"), DAR: 4.0 / 3}, {Encoded: filepath.Join("work", "c.mpg"), DAR: 16.0 / 9}}
	pages := []menu.Page{{Number: 1, Buttons: []menu.Button{{Name: "title001", Command: "g0 = 0; jump title 1;"}}}, {Number: 2, Buttons: []menu.Button{{Name: "title002", Command: "jump title 2;"}}}}
	x := AuthorXML(v, pages, s, "work")
	decoder := xml.NewDecoder(strings.NewReader(x))
	for {
		_, err := decoder.Token()
		if err != nil {
			if err.Error() != "EOF" {
				t.Fatal(err, x)
			}
			break
		}
	}
	for _, want := range []string{"a &amp; b.mpg", "aspect=\"16:9\"", "g1 = 2; call vmgm menu 2;", "if (g1 eq 2) { jump title 2; }", "button = g3;", "entry=\"root\""} {
		if !strings.Contains(x, want) {
			t.Fatal(want, x)
		}
	}
	if strings.Contains(x, "jumppad=") || strings.Contains(x, "jump titleset") {
		t.Fatal("navigation must not generate a disc-wide jump-pad command table", x)
	}
}

func TestMenuFreeFirstPlay(t *testing.T) {
	s := model.DefaultSettings(t.TempDir())
	s.NoMenu = true
	x := AuthorXML([]model.Video{{Encoded: filepath.Join("work", "a & b.mpg"), DAR: 4.0 / 3}}, nil, s, "work")
	for _, want := range []string{"<fpc>jump title 1;</fpc>", "<post>exit;</post>", "a &amp; b.mpg"} {
		if !strings.Contains(x, want) {
			t.Fatal(x)
		}
	}
	if strings.Contains(x, "<menus>") || strings.Contains(x, "page-") || strings.Contains(x, "<button") {
		t.Fatal(x)
	}
}
