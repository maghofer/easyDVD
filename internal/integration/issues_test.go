package integration

import (
	"bytes"
	"context"
	"easydvd/internal/engine"
	"easydvd/internal/media"
	"easydvd/internal/model"
	"easydvd/internal/process"
	"easydvd/internal/store"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func issueTools(t *testing.T) (media.Tools, context.Context) {
	t.Helper()
	if os.Getenv("EASYDVD_INTEGRATION") != "1" {
		t.Skip("set EASYDVD_INTEGRATION=1")
	}
	tools := media.FindTools()
	if err := tools.Require(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)
	return tools, ctx
}

func TestSelectedFilesSurviveRescanAndExclusion(t *testing.T) {
	tools, ctx := issueTools(t)
	dir := t.TempDir()
	source := filepath.Join(dir, "sources")
	if err := os.MkdirAll(filepath.Join(source, "other"), 0755); err != nil {
		t.Fatal(err)
	}
	first, second := filepath.Join(source, "clip.mp4"), filepath.Join(source, "other", "clip.mp4")
	for _, name := range []string{first, second, filepath.Join(source, "unselected.mp4")} {
		if err := process.Run(ctx, tools.FFmpeg, []string{"-v", "error", "-y", "-f", "lavfi", "-i", "testsrc2=s=320x240:r=25", "-t", "1", "-c:v", "libx264", name}, process.Options{}); err != nil {
			t.Fatal(err)
		}
	}
	broken := filepath.Join(source, "broken.mp4")
	if err := os.WriteFile(broken, []byte("not a video"), 0644); err != nil {
		t.Fatal(err)
	}
	s := model.DefaultSettings(dir)
	s.VideoEncoder = "FFmpeg"
	s.Encoders = 1
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
	if err = e.AddFiles([]string{first, second, broken}); err != nil {
		t.Fatal(err)
	}
	j := wait(t, e, func(s engine.Snapshot) bool { return s.Jobs[0].State != model.Scanning }).Jobs[0]
	if j.State != model.Blocked || len(j.Videos) != 3 {
		t.Fatal(j.State, j.Error, len(j.Videos))
	}
	var badID string
	for _, v := range j.Videos {
		if v.Error != "" {
			badID = v.ID
		}
	}
	if badID == "" {
		t.Fatal("broken fixture was not flagged")
	}
	if err = e.SetExcluded(j.ID, []string{badID}, true); err != nil {
		t.Fatal(err)
	}
	if e.Snapshot().Jobs[0].State != model.Ready {
		t.Fatal("exclusion did not enable start")
	}
	// Restart as an interrupted scan, forcing rediscovery from saved Sources.
	e.Close()
	j = e.Snapshot().Jobs[0]
	j.ScanComplete, j.State = false, model.Paused
	if err = st.Save([]model.Job{j}); err != nil {
		t.Fatal(err)
	}
	e, err = engine.New(st, s, tools)
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	e.Start()
	j = wait(t, e, func(s engine.Snapshot) bool {
		return s.Jobs[0].State == model.Complete || s.Jobs[0].State == model.Blocked
	}).Jobs[0]
	if j.State != model.Complete || len(j.Videos) != 3 || len(j.ActiveVideos()) != 2 || j.ActiveVideos()[0].ID == j.ActiveVideos()[1].ID {
		t.Fatal(j.State, j.Error, j.Videos)
	}
	if err = e.AddFiles([]string{first}); err != nil {
		t.Fatal("single file should be allowed after previous job completes", err)
	}
}

func TestSingleFileAutoplayAndPillarbox(t *testing.T) {
	tools, ctx := issueTools(t)
	dir := t.TempDir()
	source := filepath.Join(dir, "Film & café Ω")
	if err := os.MkdirAll(source, 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(source, "video.mp4")
	if err := process.Run(ctx, tools.FFmpeg, []string{"-v", "error", "-y", "-f", "lavfi", "-i", "testsrc2=s=960x720:r=25,pad=1280:720:160:0", "-t", "2", "-c:v", "libx264", path}, process.Options{}); err != nil {
		t.Fatal(err)
	}
	for _, encoder := range []string{"FFmpeg", "HCenc"} {
		for _, standard := range []string{"PAL", "NTSC"} {
			t.Run(encoder+"/"+standard, func(t *testing.T) {
				s := model.DefaultSettings(dir)
				s.VideoEncoder, s.Standard, s.NoMenu, s.KeepWork, s.Encoders = encoder, standard, true, true, 1
				if encoder == "HCenc" && tools.HCencFolder(s) == "" {
					t.Skip("HCenc runtime unavailable")
				}
				s.WorkDir = filepath.Join(dir, encoder+standard+" café Ω")
				s.OutputDir = filepath.Join(dir, encoder+standard+" output")
				st, err := store.Open(filepath.Join(s.WorkDir, "queue.db"))
				if err != nil {
					t.Fatal(err)
				}
				defer st.Close()
				e, err := engine.New(st, s, tools)
				if err != nil {
					t.Fatal(err)
				}
				defer e.Close()
				if err = e.Add(path); err != nil {
					t.Fatal(err)
				}
				e.Start()
				j := wait(t, e, func(s engine.Snapshot) bool {
					return s.Jobs[0].State == model.Complete || s.Jobs[0].State == model.Blocked
				}).Jobs[0]
				if j.State != model.Complete {
					t.Fatal(j.Error)
				}
				v := j.Videos[0]
				if v.CropLeft != 160 || v.CropRight != 160 || v.Aspect() != "4:3" {
					t.Fatal(v)
				}
				authorDir := filepath.Join(s.WorkDir, "jobs", j.ID, "generation-"+j.Generation, "disc-001")
				xml, err := os.ReadFile(filepath.Join(authorDir, "dvd.xml"))
				if err != nil || !strings.Contains(string(xml), "<fpc>jump title 1;</fpc>") || strings.Contains(string(xml), "<menus>") {
					t.Fatal(string(xml), err)
				}
				if pages, _ := filepath.Glob(filepath.Join(authorDir, "page-*")); len(pages) != 0 {
					t.Fatal("menu-free job generated menu assets", pages)
				}
				var out bytes.Buffer
				if err = process.Run(ctx, tools.FFprobe, []string{"-v", "error", "-f", "dvdvideo", "-title", "1", "-i", j.Discs[0].ISO, "-show_streams", "-show_format", "-of", "json"}, process.Options{Stdout: &out}); err != nil {
					t.Fatal(err)
				}
				var probe struct {
					Streams []struct {
						Codec  string `json:"codec_name"`
						DAR    string `json:"display_aspect_ratio"`
						Height int    `json:"height"`
					}
					Format struct {
						Duration string `json:"duration"`
					}
				}
				if err = json.Unmarshal(out.Bytes(), &probe); err != nil {
					t.Fatal(err)
				}
				if len(probe.Streams) < 2 || probe.Streams[0].Codec != "mpeg2video" || probe.Streams[0].DAR != "4:3" || probe.Streams[0].Height != s.Height() || probe.Streams[1].Codec != "ac3" {
					t.Fatal(out.String())
				}
				if duration := media.Ratio(probe.Format.Duration); duration < 1.92 || duration > 2.08 {
					t.Fatal("duration drift", duration)
				}
				if err = process.Run(ctx, tools.FFmpeg, []string{"-v", "error", "-xerror", "-f", "dvdvideo", "-title", "1", "-i", j.Discs[0].ISO, "-f", "null", "NUL"}, process.Options{}); err != nil {
					t.Fatal(err)
				}
				if encoder == "HCenc" {
					log, err := os.ReadFile(filepath.Join(filepath.Dir(v.Encoded), "hcenc.log"))
					if err != nil || !strings.Contains(string(log), "encoding - pass 2") {
						t.Fatal("no two-pass evidence", err)
					}
				}
			})
		}
	}
}

func TestWiderFramePreventsPillarboxCrop(t *testing.T) {
	tools, ctx := issueTools(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "changes.mkv")
	// The final frame contains genuine 16:9 picture content, outside the
	// otherwise consistent black borders. Sampling would miss this case.
	filter := "testsrc2=s=1280x720:r=25,drawbox=x=0:y=0:w=160:h=ih:c=black:t=fill:enable='lt(t,1.96)',drawbox=x=1120:y=0:w=160:h=ih:c=black:t=fill:enable='lt(t,1.96)'"
	if err := process.Run(ctx, tools.FFmpeg, []string{"-v", "error", "-y", "-f", "lavfi", "-i", filter, "-t", "2", "-c:v", "ffv1", path}, process.Options{}); err != nil {
		t.Fatal(err)
	}
	v := model.Video{Width: 1280, Height: 720, DAR: 16.0 / 9, FPS: 25, Duration: 2}
	v, err := tools.DetectPillarbox(ctx, v, path, nil)
	if err != nil || !v.CropChecked || v.CropLeft != 0 || v.CropRight != 0 {
		t.Fatal(v, err)
	}
	t.Log(fmt.Sprintf("Retained full 16:9 frame after checking entire clip: %s", v.Aspect()))
}

func TestTinyDetailInBorderPreventsCropping(t *testing.T) {
	tools, ctx := issueTools(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "detail.mkv")
	// A 2x2 mark is too small to change a black column's average reliably.
	// Per-pixel bounding boxes must retain it, even on one frame only.
	filter := "testsrc2=s=960x720:r=25,pad=1280:720:160:0,drawbox=x=10:y=10:w=2:h=2:c=white:t=fill:enable='gte(t,0.96)'"
	if err := process.Run(ctx, tools.FFmpeg, []string{"-v", "error", "-y", "-f", "lavfi", "-i", filter, "-t", "1", "-c:v", "ffv1", path}, process.Options{}); err != nil {
		t.Fatal(err)
	}
	v, err := tools.DetectPillarbox(ctx, model.Video{Width: 1280, Height: 720, DAR: 16.0 / 9, Duration: 1}, path, nil)
	if err != nil || v.CropLeft != 0 || v.CropRight != 0 {
		t.Fatal("border detail was removed", v, err)
	}
}

func TestNearFourThreeBordersKeepAllContent(t *testing.T) {
	tools, ctx := issueTools(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "edge-variation.mkv")
	// The picture is four pixels wider than exact 4:3. Keep those pixels and
	// fit into a 4:3 DVD frame; never retain the original 160-pixel sidebars.
	if err := process.Run(ctx, tools.FFmpeg, []string{"-v", "error", "-y", "-f", "lavfi", "-i", "testsrc2=s=964x720:r=25,pad=1280:720:158:0", "-t", "1", "-c:v", "ffv1", path}, process.Options{}); err != nil {
		t.Fatal(err)
	}
	v, err := tools.DetectPillarbox(ctx, model.Video{Width: 1280, Height: 720, DAR: 16.0 / 9, Duration: 1}, path, nil)
	if err != nil || v.CropLeft != 158 || v.CropRight != 158 || v.Aspect() != "4:3" || v.CropVersion != media.PillarboxVersion {
		t.Fatal(v, err)
	}
}

func TestHCencFourWorkers(t *testing.T) {
	tools, ctx := issueTools(t)
	dir := t.TempDir()
	s := model.DefaultSettings(dir)
	if tools.HCencFolder(s) == "" {
		t.Skip("HCenc runtime unavailable")
	}
	s.Encoders = 4
	source := filepath.Join(dir, "sources")
	if err := os.Mkdir(source, 0755); err != nil {
		t.Fatal(err)
	}
	first := filepath.Join(source, "1.mp4")
	if err := process.Run(ctx, tools.FFmpeg, []string{"-v", "error", "-y", "-f", "lavfi", "-i", "testsrc2=s=320x240:r=25", "-t", "1", "-c:v", "libx264", first}, process.Options{}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(first)
	if err != nil {
		t.Fatal(err)
	}
	for n := 2; n <= 4; n++ {
		if err := os.WriteFile(filepath.Join(source, fmt.Sprintf("%d.mp4", n)), data, 0644); err != nil {
			t.Fatal(err)
		}
	}
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
	if err = e.Add(source); err != nil {
		t.Fatal(err)
	}
	e.Start()
	j := wait(t, e, func(s engine.Snapshot) bool {
		return s.Jobs[0].State == model.Complete || s.Jobs[0].State == model.Blocked
	}).Jobs[0]
	if j.State != model.Complete || len(j.Videos) != 4 || len(j.Discs) != 1 {
		t.Fatal(j.State, j.Error)
	}
}
