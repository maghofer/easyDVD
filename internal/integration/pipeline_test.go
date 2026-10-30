// Package integration exercises the actual external toolchain. Enable with EASYDVD_INTEGRATION=1.
package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"easydvd/internal/app"
	"easydvd/internal/engine"
	"easydvd/internal/media"
	"easydvd/internal/model"
	"easydvd/internal/process"
	"easydvd/internal/store"
)

func TestMain(m *testing.M) {
	if ok, err := app.Worker(os.Args[1:]); ok {
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}
func wait(t *testing.T, e *engine.Engine, predicate func(engine.Snapshot) bool) engine.Snapshot {
	t.Helper()
	timeout := time.NewTimer(90 * time.Second)
	defer timeout.Stop()
	tick := time.NewTicker(20 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-timeout.C:
			t.Fatalf("pipeline timed out: %+v", e.Snapshot())
		case <-tick.C:
			s := e.Snapshot()
			if s.Error != "" {
				t.Fatal(s.Error)
			}
			if predicate(s) {
				return s
			}
		}
	}
}
func TestPipeline(t *testing.T) {
	if os.Getenv("EASYDVD_INTEGRATION") != "1" {
		t.Skip("set EASYDVD_INTEGRATION=1 to run real encoders and DVD authoring")
	}
	tools := media.FindTools()
	if err := tools.Require(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	dir := t.TempDir()
	source := filepath.Join(dir, "Film & café")
	if err := os.MkdirAll(filepath.Join(source, "nested"), 0755); err != nil {
		t.Fatal(err)
	}
	fixtures := []struct {
		name, filter string
		audio        bool
	}{{"5694-001.mp4", "testsrc2=size=640x480:rate=25", true}, {"5694-002.mp4", "testsrc2=size=720x576:rate=50", false}, {"nested/5694-010.mp4", "testsrc2=size=1280x720:rate=30000/1001", false}}
	original := map[string]string{}
	for i, f := range fixtures {
		args := []string{"-v", "error", "-y", "-f", "lavfi", "-i", f.filter}
		if f.audio {
			args = append(args, "-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000")
		}
		args = append(args, "-t", "2", "-c:v", "libx264")
		if i == 1 {
			args = append(args, "-vf", "tinterlace=mode=interleave_top,setsar=16/15", "-flags", "+ilme+ildct", "-x264-params", "tff=1")
		}
		args = append(args, filepath.Join(source, f.name))
		if err := process.Run(ctx, tools.FFmpeg, args, process.Options{}); err != nil {
			t.Fatal(err)
		}
		h, err := media.HashFile(ctx, filepath.Join(source, f.name))
		if err != nil {
			t.Fatal(err)
		}
		original[f.name] = h
	}
	for _, tc := range []struct{ profile, standard string }{
		{model.ProfileBalanced, "PAL"}, {model.ProfileBalanced, "NTSC"},
		{model.ProfileQuality, "PAL"}, {model.ProfileQuality, "NTSC"},
	} {
		profile, standard := tc.profile, tc.standard
		t.Run(profile+"/"+standard, func(t *testing.T) {
			s := model.DefaultSettings(dir)
			s.Standard = standard
			s.EncodingProfile = profile
			s.OutputDir = filepath.Join(dir, "output-"+profile+"-"+standard)
			s.WorkDir = filepath.Join(dir, "work-"+profile+"-"+standard)
			s.TitlesPerPage = 2
			s.KeepWork = true
			s.Encoders = 1
			if standard == "NTSC" {
				s.Menu.Aspect = "16:9"
				s.Layout = "Thumbnails"
				s.StageLocally = false
				s.Media = "DVD-9"
				s.Packing = "Space efficiency"
			}
			st, err := store.Open(filepath.Join(dir, profile+"-"+standard+".db"))
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
			snap := wait(t, e, func(s engine.Snapshot) bool { return s.Jobs[0].State != model.Scanning })
			if snap.Jobs[0].State != model.Ready {
				t.Fatal(snap.Jobs[0].Error)
			}
			if len(snap.Jobs[0].Videos) != 3 || !strings.HasSuffix(snap.Jobs[0].Videos[2].Path, "5694-010.mp4") {
				t.Fatal("discovery/order failed")
			}
			// A failed folder must not prevent the valid set behind it from running.
			if err = e.Add(filepath.Join(dir, "missing")); err != nil {
				t.Fatal(err)
			}
			e.Start()
			if standard == "PAL" {
				partial := wait(t, e, func(s engine.Snapshot) bool {
					j := s.Jobs[0]
					for _, v := range j.Videos {
						if v.Encoded != "" {
							return true
						}
					}
					return j.State == model.Blocked || j.State == model.Complete
				})
				if partial.Jobs[0].State == model.Blocked {
					t.Fatal(partial.Jobs[0].Error)
				}
				e.Pause()
				paused := wait(t, e, func(s engine.Snapshot) bool {
					return s.Jobs[0].State == model.Paused || s.Jobs[0].State == model.Complete || s.Jobs[0].State == model.Blocked
				})
				if paused.Jobs[0].State == model.Paused {
					var cached model.Video
					for _, v := range paused.Jobs[0].Videos {
						if v.Encoded != "" {
							cached = v
							break
						}
					}
					info, err := os.Stat(cached.Encoded)
					if err != nil {
						t.Fatal(err)
					}
					e.Close()
					e, err = engine.New(st, s, tools)
					if err != nil {
						t.Fatal(err)
					}
					defer e.Close()
					if e.Snapshot().Jobs[0].State != model.Paused {
						t.Fatal("paused checkpoint was not restored")
					}
					e.Start()
					defer func() {
						after, err := os.Stat(cached.Encoded)
						if err != nil || !after.ModTime().Equal(info.ModTime()) {
							t.Error("completed encode was not reused", err)
						}
					}()
				}
			}
			snap = wait(t, e, func(s engine.Snapshot) bool {
				return s.Jobs[0].State == model.Complete || s.Jobs[0].State == model.Blocked
			})
			j := snap.Jobs[0]
			if j.State != model.Complete {
				t.Fatal(j.Error)
			}
			if len(j.Discs) != 1 {
				t.Fatal(j.Discs)
			}
			if standard == "NTSC" {
				page := filepath.Join(s.WorkDir, "jobs", j.ID, "generation-"+j.Generation, "disc-001", "page-001.mpg")
				var info bytes.Buffer
				if err = process.Run(ctx, tools.FFprobe, []string{"-v", "error", "-show_streams", "-of", "json", page}, process.Options{Stdout: &info}); err != nil {
					t.Fatal(err)
				}
				var result struct {
					Streams []struct {
						Type string `json:"codec_type"`
						DAR  string `json:"display_aspect_ratio"`
					}
				}
				if err = json.Unmarshal(info.Bytes(), &result); err != nil {
					t.Fatal(err)
				}
				count := 0
				wide := false
				for _, stream := range result.Streams {
					if stream.Type == "subtitle" {
						count++
					}
					if stream.Type == "video" && stream.DAR == "16:9" {
						wide = true
					}
				}
				if !wide || count != 2 {
					t.Fatal("widescreen menu must have wide and letterbox highlight streams", info.String())
				}
			}
			hash, err := media.HashFile(ctx, j.Discs[0].ISO)
			if err != nil || hash != j.Discs[0].SHA256 {
				t.Fatal("published hash mismatch", err)
			}
			for n := 1; n <= 3; n++ {
				var b strings.Builder
				args := []string{"-v", "error", "-f", "dvdvideo", "-title", fmt.Sprint(n), "-i", j.Discs[0].ISO, "-show_streams", "-show_format", "-of", "json"}
				if err = process.Run(ctx, tools.FFprobe, args, process.Options{Stdout: &b}); err != nil {
					t.Fatal(err)
				}
				var p struct {
					Streams []struct {
						Codec      string `json:"codec_name"`
						Height     int    `json:"height"`
						Channels   int    `json:"channels"`
						DAR        string `json:"display_aspect_ratio"`
						Field      string `json:"field_order"`
						Pixel      string `json:"pix_fmt"`
						Rate       string `json:"r_frame_rate"`
						BitRate    string `json:"bit_rate"`
						SampleRate string `json:"sample_rate"`
					} `json:"streams"`
					Format struct {
						Duration string `json:"duration"`
					} `json:"format"`
				}
				if err = json.Unmarshal([]byte(b.String()), &p); err != nil {
					t.Fatal(err)
				}
				if len(p.Streams) < 2 || p.Streams[0].Codec != "mpeg2video" || p.Streams[0].Height != s.Height() || p.Streams[1].Codec != "ac3" || p.Streams[1].Channels != 2 {
					t.Fatal(b.String())
				}
				if p.Streams[0].Pixel != "yuv420p" || media.Ratio(p.Streams[0].Rate) != media.Ratio(s.FPS()) || p.Streams[1].BitRate != fmt.Sprint(s.AudioBitrate()) || p.Streams[1].SampleRate != "48000" {
					t.Fatal("DVD profile, frame rate or audio constraints violated", b.String())
				}
				// dvdvideo exposes IFO properties but does not report the MPEG-2
				// sequence profile/level; inspect the encoded title itself for these.
				var sequence bytes.Buffer
				if err = process.Run(ctx, tools.FFprobe, []string{"-v", "error", "-select_streams", "v:0", "-show_entries", "stream=profile,level", "-of", "json", j.Videos[n-1].Encoded}, process.Options{Stdout: &sequence}); err != nil {
					t.Fatal(err)
				}
				var seq struct {
					Streams []struct {
						Profile string
						Level   int
					}
				}
				if err = json.Unmarshal(sequence.Bytes(), &seq); err != nil || len(seq.Streams) != 1 || seq.Streams[0].Profile != "Main" || seq.Streams[0].Level != 8 {
					t.Fatal("MPEG-2 Main Profile / Main Level required", sequence.String(), err)
				}
				if n == 3 && p.Streams[0].DAR != "16:9" {
					t.Fatal("widescreen lost")
				}
				if n == 2 {
					if standard == "PAL" && p.Streams[0].Field == "progressive" {
						t.Fatal("matching interlacing lost")
					}
					if standard == "NTSC" && p.Streams[0].Field != "progressive" {
						t.Fatal("cross-standard footage was not deinterlaced")
					}
				}
				duration := media.Ratio(p.Format.Duration)
				if duration < 1.92 || duration > 2.08 {
					t.Fatal("duration drift", n, duration)
				}
			}
			if _, err = os.Stat(filepath.Join(engine.PreviewDir(j), "disc-001", "page-002-preview.png")); err != nil {
				t.Fatal("missing second menu page", err)
			}
			entries, err := os.ReadDir(j.OutputPath)
			if err != nil || len(entries) != 1 || entries[0].Name() != "DVD-001-of-001.iso" {
				t.Fatal("finished output must contain only the ISO", entries, err)
			}
			for _, record := range []string{e.JobLogPath(j.ID), e.ManifestPath(j.ID)} {
				if info, err := os.Stat(record); err != nil || info.Size() == 0 {
					t.Fatal("missing persistent job record", record, err)
				}
			}
			for name, h := range original {
				after, err := media.HashFile(ctx, filepath.Join(source, name))
				if err != nil || after != h {
					t.Fatal("source changed", name, err)
				}
			}
			// Simulate a restart after the atomic ISO-folder rename but before
			// the queue checkpoint. Recovery must use the local manifest,
			// without needing a manifest beside the finished ISO.
			e.Close()
			j.State = model.Running
			if err = st.Save([]model.Job{j}); err != nil {
				t.Fatal(err)
			}
			e, err = engine.New(st, s, tools)
			if err != nil {
				t.Fatal(err)
			}
			defer e.Close()
			e.Start()
			recovered := wait(t, e, func(s engine.Snapshot) bool {
				return s.Jobs[0].State == model.Complete || s.Jobs[0].State == model.Blocked
			}).Jobs[0]
			if recovered.State != model.Complete || recovered.Discs[0].SHA256 != j.Discs[0].SHA256 {
				t.Fatal("ISO-only publication recovery failed", recovered.Error)
			}
		})
	}
}
