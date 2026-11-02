package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"easydvd/internal/engine"
	"easydvd/internal/media"
	"easydvd/internal/model"
	"easydvd/internal/process"
	"easydvd/internal/store"
)

func blurayTools(t *testing.T) (media.Tools, context.Context) {
	t.Helper()
	if os.Getenv("EASYDVD_BLURAY_INTEGRATION") != "1" {
		t.Skip("set EASYDVD_BLURAY_INTEGRATION=1 and prepare the private Blu-ray runtime")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	t.Cleanup(cancel)
	tools := media.FindTools()
	s := model.DefaultSettings(t.TempDir())
	s.DiscFormat = "Blu-ray"
	s.Media = "BD-25"
	if err := tools.RequireFor(ctx, s); err != nil {
		t.Fatal(err)
	}
	return tools, ctx
}

func bluraySource(t *testing.T, ctx context.Context, tools media.Tools, path, size, rate, filter, params, audio string) {
	t.Helper()
	if params != "" {
		params += ":"
	}
	params += "force-cfr=1:colorprim=bt709:transfer=bt709:colormatrix=bt709:fullrange=off"
	args := []string{"-hide_banner", "-v", "error", "-nostdin", "-y", "-f", "lavfi", "-i", fmt.Sprintf("testsrc2=s=%s:r=%s:d=2", size, rate)}
	if audio != "" {
		args = append(args, "-f", "lavfi", "-i", "anullsrc=r=48000:cl=5.1:d=2")
	}
	if filter != "" {
		args = append(args, "-vf", filter)
	}
	args = append(args, "-c:v", "libx264", "-preset", "medium", "-crf", "20", "-profile:v", "high", "-level:v", "4.1", "-maxrate", "38000k", "-bufsize", "30000k", "-x264-params", params, "-colorspace", "bt709", "-color_primaries", "bt709", "-color_trc", "bt709", "-color_range", "tv")
	if audio != "" {
		args = append(args, "-c:a", audio, "-ac", "6", "-b:a", "448k")
	}
	args = append(args, path)
	if err := process.Run(ctx, tools.FFmpeg, args, process.Options{}); err != nil {
		t.Fatal(err)
	}
}

func TestBlurayEncoding(t *testing.T) {
	tools, ctx := blurayTools(t)
	for _, tc := range []struct {
		name, size, rate, filter, params, audio, mode string
		copy                                          bool
		channels                                      int
	}{
		{"verified-copy", "1920x1080", "24", "", "bluray-compat=1:aud=1:slices=4:nal-hrd=vbr:keyint=24:min-keyint=1:open-gop=0:bframes=0:ref=1:weightp=0", "ac3", "1080", true, 6},
		{"missing-hrd", "1920x1080", "24", "", "aud=1:slices=4:nal-hrd=none:bframes=0:ref=1:weightp=0", "", "1080", false, 2},
		{"long-gop", "1920x1080", "24", "", "bluray-compat=1:aud=1:slices=4:nal-hrd=vbr:keyint=240:min-keyint=240:scenecut=0:open-gop=0:bframes=0:ref=1:weightp=0", "", "1080", false, 2},
		{"b-pictures", "1920x1080", "24", "", "bluray-compat=1:aud=1:slices=4:nal-hrd=vbr:keyint=24:bframes=3", "", "1080", false, 2},
		{"native-sd", "720x576", "25", "setsar=16/15", "", "", "576", false, 2},
		{"native-ntsc", "720x480", "30000/1001", "setsar=8/9", "", "", "480", false, 2},
		{"720p50-surround", "1280x720", "50", "", "", "aac", "720", false, 6},
		{"1080p50-motion", "1920x1080", "50", "", "", "", "1080", false, 2},
		{"1080p5994-motion", "1920x1080", "60000/1001", "", "", "", "1080", false, 2},
		{"true-interlaced", "1920x1080", "50", "tinterlace=interleave_top", "interlaced=1:tff=1", "", "1080", false, 2},
		{"film23976", "1920x1080", "24000/1001", "", "", "", "1080", false, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			src := filepath.Join(dir, "Film café.mp4")
			bluraySource(t, ctx, tools, src, tc.size, tc.rate, tc.filter, tc.params, tc.audio)
			v, err := tools.Probe(ctx, src, filepath.Base(src))
			if err != nil {
				t.Fatal(err)
			}
			s := model.DefaultSettings(dir)
			s.DiscFormat = "Blu-ray"
			s.Media = "BD-25"
			s.Encoders = 1
			if tc.name == "native-sd" {
				s.EncodingProfile = model.ProfileQuality
			}
			var log bytes.Buffer
			encoded, err := tools.Encode(ctx, v, s, src, filepath.Join(dir, "encoded"), &log, nil)
			if err != nil {
				t.Fatalf("%v\n%s", err, log.String())
			}
			if encoded.Bluray.CopyVideo != tc.copy || encoded.Bluray.Mode != tc.mode || encoded.Bluray.Channels != tc.channels {
				t.Fatalf("unexpected plan: %+v\n%s", encoded.Bluray, log.String())
			}
			if strings.Contains(tc.name, "motion") {
				prepared, err := tools.Probe(ctx, encoded.Encoded, "prepared")
				if err != nil {
					t.Fatal(err)
				}
				if prepared.FieldOrder != "tt" && prepared.FieldOrder != "tb" {
					t.Fatal("high-rate footage was not interlaced", prepared.FieldOrder)
				}
				var count bytes.Buffer
				if err = process.Run(ctx, tools.FFmpeg, []string{"-v", "error", "-i", encoded.Encoded, "-map", "0:v:0", "-vf", "bwdif=mode=send_field:parity=auto", "-progress", "pipe:1", "-f", "null", "-"}, process.Options{Stdout: &count}); err != nil {
					t.Fatal(err)
				}
				want := "frame=100"
				if strings.Contains(tc.name, "5994") {
					want = "frame=120"
				}
				if !strings.Contains(count.String(), want) {
					t.Fatal("field cadence was lost", count.String())
				}
			}
			if !media.ValidArtifact(ctx, encoded, s) {
				t.Fatal("checkpoint failed validation")
			}
			s.Menu.FontSize++
			if !media.ValidArtifact(ctx, encoded, s) {
				t.Fatal("menu invalidated encode")
			}
			s.EncodingProfile = map[string]string{model.ProfileQuality: model.ProfileBalanced, model.ProfileBalanced: model.ProfileQuality}[s.Profile()]
			if media.ValidArtifact(ctx, encoded, s) {
				t.Fatal("profile failed to invalidate encode")
			}
			if tc.copy {
				digest := func(path string) string {
					var b bytes.Buffer
					if err := process.Run(ctx, tools.FFmpeg, []string{"-v", "error", "-i", path, "-map", "0:v:0", "-c:v", "copy", "-bsf:v", "h264_mp4toannexb", "-f", "hash", "-hash", "sha256", "-"}, process.Options{Stdout: &b}); err != nil {
						t.Fatal(err)
					}
					return b.String()
				}
				if digest(src) != digest(encoded.Encoded) {
					t.Fatal("copied AVC payload changed")
				}
			}
		})
	}
}

func TestBlurayAuthorCancellationAndRetry(t *testing.T) {
	tools, ctx := blurayTools(t)
	dir := t.TempDir()
	src := filepath.Join(dir, "source.mp4")
	bluraySource(t, ctx, tools, src, "720x576", "25", "", "", "ac3")
	v, err := tools.Probe(ctx, src, "source.mp4")
	if err != nil {
		t.Fatal(err)
	}
	s := model.DefaultSettings(dir)
	s.DiscFormat = "Blu-ray"
	s.Media = "BD-25"
	s.Encoders = 1
	s.NoMenu = true
	v, err = tools.Encode(ctx, v, s, src, filepath.Join(dir, "asset"), os.Stderr, nil)
	if err != nil {
		t.Fatal(err)
	}
	j := model.Job{Title: "Retry", Settings: s, Videos: []model.Video{v}}
	disc := model.Disc{Number: 1, VideoIDs: []string{v.ID}}
	stage := func(name string) error {
		if name == "Creating Blu-ray ISO" {
			return context.Canceled
		}
		return nil
	}
	authorDir := filepath.Join(dir, "author")
	if _, err = tools.Author(ctx, j, disc, 1, authorDir, os.Stderr, stage); !errors.Is(err, context.Canceled) {
		t.Fatal("helper did not propagate cancellation", err)
	}
	if _, err = tools.Author(ctx, j, disc, 1, authorDir, os.Stderr, func(string) error { return nil }); err != nil {
		t.Fatal("retry did not replace interrupted authoring", err)
	}
	if !media.ValidArtifact(ctx, v, s) {
		t.Fatal("author retry changed prepared media")
	}
}

func TestBlurayAdditionalInputs(t *testing.T) {
	tools, ctx := blurayTools(t)
	for _, kind := range []string{"prores-mono", "vfr-short-audio", "sdr-wide-gamut", "hdr-rejected"} {
		t.Run(kind, func(t *testing.T) {
			dir := t.TempDir()
			ext := ".mp4"
			if kind == "prores-mono" {
				ext = ".mov"
			}
			src := filepath.Join(dir, "Film café "+strings.Repeat("long-", 24)+ext)
			args := []string{"-v", "error", "-nostdin", "-y", "-f", "lavfi", "-i", "testsrc2=s=640x480:r=25:d=2", "-f", "lavfi", "-i", "anullsrc=r=48000:cl=mono:d=0.25"}
			switch kind {
			case "prores-mono":
				args = append(args, "-c:v", "prores_ks", "-profile:v", "1", "-pix_fmt", "yuv422p10le", "-c:a", "pcm_s16le")
			case "vfr-short-audio":
				args = append(args, "-vf", "setpts=PTS+0.08*floor(N/5)/TB", "-fps_mode", "vfr", "-c:v", "libx264", "-c:a", "aac")
			case "hdr-rejected":
				args = append(args, "-c:v", "libx264", "-x264-params", "transfer=smpte2084:colorprim=bt2020:colormatrix=bt2020nc", "-c:a", "aac")
			case "sdr-wide-gamut":
				args = append(args, "-c:v", "libx264", "-x264-params", "transfer=bt709:colorprim=bt2020:colormatrix=bt2020nc", "-c:a", "aac")
			}
			args = append(args, src)
			if err := process.Run(ctx, tools.FFmpeg, args, process.Options{}); err != nil {
				t.Fatal(err)
			}
			v, err := tools.Probe(ctx, src, filepath.Base(src))
			if kind == "hdr-rejected" {
				if err == nil || !strings.Contains(err.Error(), "HDR") {
					t.Fatal("HDR was not rejected", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			s := model.DefaultSettings(dir)
			s.DiscFormat = "Blu-ray"
			s.Media = "BD-25"
			s.NoMenu = true
			s.Encoders = 1
			var log bytes.Buffer
			v, err = tools.Encode(ctx, v, s, src, filepath.Join(dir, "asset"), &log, nil)
			if err != nil {
				t.Fatal(err, log.String())
			}
			if v.Bluray.CopyVideo || v.Bluray.Channels != 1 {
				t.Fatal(v.Bluray)
			}
			prepared, err := tools.Probe(ctx, v.Encoded, "prepared")
			if err != nil {
				t.Fatal(err)
			}
			if prepared.Source.AudioDuration > 0 && prepared.Source.AudioDuration < v.Duration-.05 {
				t.Fatal("short audio was not padded", prepared.Source.AudioDuration, v.Duration)
			}
			j := model.Job{Title: "Mono", Settings: s, Videos: []model.Video{v}}
			iso, err := tools.Author(ctx, j, model.Disc{Number: 1, VideoIDs: []string{v.ID}}, 1, filepath.Join(dir, "author"), &log, func(string) error { return nil })
			if err != nil {
				t.Fatal(err, log.String())
			}
			if reader := os.Getenv("EASYDVD_BLURAY_READER"); reader != "" {
				var b bytes.Buffer
				if err = process.Run(ctx, reader, []string{iso, "inspect", "1"}, process.Options{Stdout: &b}); err != nil {
					t.Fatal(err)
				}
				var info struct {
					Audio int `json:"audio"`
				}
				if json.Unmarshal(bytes.TrimSpace(b.Bytes()), &info) != nil || info.Audio != 1 {
					t.Fatal("mono signaling was lost", b.String())
				}
			}
		})
	}
}

func TestBlurayQueuePipeline(t *testing.T) {
	tools, ctx := blurayTools(t)
	dir := t.TempDir()
	source := filepath.Join(dir, "Mixed café")
	if err := os.Mkdir(source, 0755); err != nil {
		t.Fatal(err)
	}
	for n, tc := range []struct{ size, rate string }{{"720x576", "25"}, {"1280x720", "50"}, {"1920x1080", "24"}} {
		bluraySource(t, ctx, tools, filepath.Join(source, fmt.Sprintf("%d.mp4", n+1)), tc.size, tc.rate, "", "", "aac")
	}
	s := model.DefaultSettings(dir)
	s.DiscFormat = "Blu-ray"
	s.Media = "BD-25"
	s.Encoders = 1
	s.KeepWork = true
	s.TitlesPerPage = 2
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
	wait(t, e, func(s engine.Snapshot) bool { return s.Jobs[0].State != model.Scanning })
	e.Start()
	partial := wait(t, e, func(s engine.Snapshot) bool {
		for _, v := range s.Jobs[0].Videos {
			if v.Encoded != "" {
				return true
			}
		}
		return s.Jobs[0].State == model.Blocked || s.Jobs[0].State == model.Complete
	})
	if partial.Jobs[0].State == model.Blocked {
		t.Fatal(partial.Jobs[0].Error)
	}
	e.Pause()
	paused := wait(t, e, func(s engine.Snapshot) bool {
		return s.Jobs[0].State == model.Paused || s.Jobs[0].State == model.Complete || s.Jobs[0].State == model.Blocked
	})
	if paused.Jobs[0].State == model.Blocked {
		t.Fatal(paused.Jobs[0].Error)
	}
	var retained model.Video
	for _, v := range paused.Jobs[0].Videos {
		if v.Encoded != "" {
			retained = v
			break
		}
	}
	before, err := os.Stat(retained.Encoded)
	if err != nil {
		t.Fatal(err)
	}
	e.Close()
	e, err = engine.New(st, s, tools)
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	e.Start()
	snap := wait(t, e, func(s engine.Snapshot) bool {
		return s.Jobs[0].State == model.Complete || s.Jobs[0].State == model.Blocked
	})
	j := snap.Jobs[0]
	if j.State != model.Complete {
		t.Fatal(j.Error)
	}
	after, err := os.Stat(retained.Encoded)
	if err != nil || !after.ModTime().Equal(before.ModTime()) {
		t.Fatal("resume re-encoded completed media", err)
	}
	if len(j.Discs) != 1 || !strings.HasSuffix(j.Discs[0].ISO, "BD-001-of-001.iso") {
		t.Fatal(j.Discs)
	}
	for _, v := range j.Videos {
		if !media.ValidArtifact(ctx, v, j.Settings) {
			t.Fatal("invalid queue checkpoint", v.Relative)
		}
	}
	reader := os.Getenv("EASYDVD_BLURAY_READER")
	if reader != "" {
		var out bytes.Buffer
		if err = process.Run(ctx, reader, []string{j.Discs[0].ISO, "inspect", "6"}, process.Options{Stdout: &out}); err != nil {
			t.Fatal(err)
		}
		lines := strings.Split(strings.TrimSpace(out.String()), "\n")
		if len(lines) != 6 {
			t.Fatal(out.String())
		}
		var first struct {
			Format int `json:"format"`
		}
		if json.Unmarshal([]byte(lines[0]), &first) != nil || first.Format != 2 {
			t.Fatal(out.String())
		}
		out.Reset()
		if err = process.Run(ctx, reader, []string{j.Discs[0].ISO, "play", "13,13,16"}, process.Options{Stdout: &out}); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out.String(), `"playlist":4`) {
			t.Fatal("Play All was not reached", out.String())
		}
	}
	finished := wait(t, e, func(s engine.Snapshot) bool { return !s.Enabled && !s.Busy[j.ID] })
	if finished.Jobs[0].Progress != 1 || finished.Jobs[0].ElapsedSeconds <= 0 || finished.ActiveFiles != 0 {
		t.Fatal("terminal queue status is stale", finished)
	}
	previousISO, previousHash := j.Discs[0].ISO, j.Discs[0].SHA256
	if err = e.ClearRenderStatus([]string{j.ID}); err != nil {
		t.Fatal(err)
	}
	e.Start()
	rerun := wait(t, e, func(s engine.Snapshot) bool { return !s.Enabled && !s.Busy[j.ID] })
	next := rerun.Jobs[0]
	if next.State != model.Complete || next.RenderRevision != 1 || next.Discs[0].ISO == previousISO || next.Progress != 1 {
		t.Fatal("fresh render failed", next)
	}
	if hash, err := media.HashFile(ctx, previousISO); err != nil || hash != previousHash {
		t.Fatal("previous ISO was changed", err)
	}
	for i, v := range next.Videos {
		if v.Encoded == j.Videos[i].Encoded || !strings.Contains(v.Encoded, "render-001") {
			t.Fatal("reset reused previous prepared media", v.Encoded)
		}
	}
}
