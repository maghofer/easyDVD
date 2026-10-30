package integration

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"easydvd/internal/engine"
	"easydvd/internal/model"
	"easydvd/internal/process"
	"easydvd/internal/store"
)

func TestAutomaticMenuDefaultsAndMultiVideoAutoplay(t *testing.T) {
	tools, ctx := issueTools(t)
	dir := t.TempDir()
	var paths []string
	for i, size := range []string{"320x240", "426x240", "320x240"} {
		path := filepath.Join(dir, fmt.Sprintf("film-%d.mp4", i+1))
		if err := process.Run(ctx, tools.FFmpeg, []string{"-v", "error", "-y", "-f", "lavfi", "-i", "testsrc2=s=" + size + ":r=25", "-t", "2", "-c:v", "libx264", path}, process.Options{}); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, path)
	}
	s := model.DefaultSettings(dir)
	s.KeepWork, s.Encoders = true, 1
	st, err := store.Open(filepath.Join(dir, "queue.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	e, err := engine.New(st, s, tools)
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	if err := e.AddFiles(paths); err != nil {
		t.Fatal(err)
	}
	j := wait(t, e, func(s engine.Snapshot) bool { return s.Jobs[0].ScanComplete }).Jobs[0]
	if j.Settings.NoMenu {
		t.Fatal("multi-video job did not default to a menu")
	}
	next := j.Settings
	next.NoMenu = true
	if err := e.SetJobSettings(j.ID, next); err != nil {
		t.Fatal(err)
	}
	e.Start()
	j = wait(t, e, func(s engine.Snapshot) bool {
		return s.Jobs[0].State == model.Complete || s.Jobs[0].State == model.Blocked
	}).Jobs[0]
	if j.State != model.Complete || len(j.Discs) != 1 {
		t.Fatal("multi-video autoplay failed", j.Error)
	}
	authorDir := filepath.Join(s.WorkDir, "jobs", j.ID, "generation-"+j.Generation, "disc-001")
	if pages, _ := filepath.Glob(filepath.Join(authorDir, "page-*")); len(pages) != 0 {
		t.Fatal("autoplay rendered menu media", pages)
	}
	xml, err := os.ReadFile(filepath.Join(authorDir, "dvd.xml"))
	if err != nil || strings.Contains(string(xml), "<button") || strings.Count(string(xml), "<titleset>") != 3 {
		t.Fatal(string(xml), err)
	}
	for i := 1; i <= 3; i++ {
		if err := process.Run(ctx, tools.FFmpeg, []string{"-v", "error", "-xerror", "-f", "dvdvideo", "-title", fmt.Sprint(i), "-i", j.Discs[0].ISO, "-f", "null", "NUL"}, process.Options{}); err != nil {
			t.Fatal(err)
		}
	}
	if nav, vlc := os.Getenv("EASYDVD_NAVCHECK"), os.Getenv("EASYDVD_VLC_DIR"); nav != "" && vlc != "" {
		var output bytes.Buffer
		if err := process.Run(ctx, nav, []string{vlc, j.Discs[0].ISO, "--autoplay", "3"}, process.Options{Stdout: &output, Log: &output}); err != nil {
			t.Fatal(output.String(), err)
		}
		t.Log(output.String())
	}
	e.Pause()
	if err := e.AddFiles(paths[:1]); err != nil {
		t.Fatal(err)
	}
	second := wait(t, e, func(s engine.Snapshot) bool { return len(s.Jobs) == 2 && s.Jobs[1].ScanComplete }).Jobs[1]
	if !second.Settings.NoMenu || second.NoMenuOverride != nil {
		t.Fatal("single-video job did not default to automatic playback")
	}
	next = second.Settings
	next.NoMenu = false
	if err := e.SetJobSettings(second.ID, next); err != nil {
		t.Fatal(err)
	}
	if e.Snapshot().Jobs[1].Settings.NoMenu {
		t.Fatal("single-video menu override was not accepted")
	}
}
