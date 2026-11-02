package media

import (
	"context"
	"strings"
	"testing"

	"easydvd/internal/model"
)

func TestBlurayTargetPreservesRastersAndMotion(t *testing.T) {
	for _, tc := range []struct {
		w, h              int
		fps               float64
		field, mode, rate string
		interlace, weave  bool
	}{
		{720, 576, 25, "progressive", "576", "25/1", false, false},
		{720, 480, 30000.0 / 1001, "bb", "480", "30000/1001", true, false},
		{960, 720, 24, "progressive", "720", "24/1", false, false},
		{1280, 720, 50, "progressive", "720", "50/1", false, false},
		{1920, 1080, 24, "progressive", "1080", "24/1", false, false},
		{1920, 1080, 24000.0 / 1001, "progressive", "1080", "24000/1001", false, false},
		{1920, 1080, 50, "progressive", "1080", "25/1", true, true},
		{1920, 1080, 60000.0 / 1001, "progressive", "1080", "30000/1001", true, true},
		{1920, 1080, 25, "tt", "1080", "25/1", true, false},
		{3840, 2160, 24, "progressive", "1080", "24/1", false, false},
		{2048, 1556, 18, "progressive", "1080", "24/1", false, false},
	} {
		v := model.Video{Width: tc.w, Height: tc.h, FPS: tc.fps, FieldOrder: tc.field, DAR: 16.0 / 9, AudioStream: -1}
		p := BlurayTarget(v)
		if p.Mode != tc.mode || blurayFPS(p) != tc.rate || p.Interlaced != tc.interlace || p.Weave != tc.weave {
			t.Fatalf("%+v => %+v", tc, p)
		}
		filter := BlurayFilter(v, p)
		if tc.weave && !strings.Contains(filter, "tinterlace") {
			t.Fatal("lost high-rate motion", filter)
		}
		if tc.field == "bb" && !strings.Contains(filter, "fieldorder=tff") {
			t.Fatal("bottom-field order lost", filter)
		}
	}
}

func TestBlurayAudioAndProfileDecisions(t *testing.T) {
	s := model.DefaultSettings(t.TempDir())
	s.DiscFormat = "Blu-ray"
	s.Media = "BD-25"
	v := model.Video{Width: 1920, Height: 1080, FPS: 24, DAR: 16.0 / 9, Duration: 10, AudioStream: 1, Source: model.StreamMetadata{AudioCodec: "ac3", AudioRate: 48000, AudioChannels: 6, AudioBitrate: 640000, AudioDuration: 10}}
	p := BlurayTarget(v)
	if !p.CopyAudio || p.Channels != 6 {
		t.Fatal(p)
	}
	v.Source.AudioDuration = 2
	if BlurayTarget(v).CopyAudio {
		t.Fatal("short audio was copied instead of padded")
	}
	v.Source.AudioCodec = "aac"
	v.Source.AudioChannels = 8
	v.Bluray = BlurayTarget(v)
	args := strings.Join(BlurayEncodeArgs(v, s, "source", "output"), " ")
	for _, want := range []string{"-preset medium", "-crf 20", "-ac 6", "-b:a 640k", "-af apad", "-mpegts_m2ts_mode 1"} {
		if !strings.Contains(args, want) {
			t.Fatal(want, args)
		}
	}
	s.EncodingProfile = model.ProfileQuality
	args = strings.Join(BlurayEncodeArgs(v, s, "source", "output"), " ")
	if !strings.Contains(args, "-preset slow -crf 18") || strings.Contains(args, "-pass ") {
		t.Fatal(args)
	}
	v.Bluray.CopyVideo = true
	args = strings.Join(BlurayEncodeArgs(v, s, "source", "output"), " ")
	if strings.Contains(args, "-vf ") || !strings.Contains(args, "-c:v copy") {
		t.Fatal(args)
	}
}

func TestBlurayVerifierRejectsMetadataOnlyClaims(t *testing.T) {
	v := model.Video{Width: 1920, Height: 1080, FPS: 24, DAR: 16.0 / 9, Source: model.StreamMetadata{Codec: "h264", Profile: "High", Level: 41, PixelFormat: "yuv420p"}}
	v.Bluray = BlurayTarget(v)
	copy, reason, err := (Tools{}).verifyBlurayAVC(context.Background(), v, "unused")
	if err != nil || copy || !strings.Contains(reason, "signaling") {
		t.Fatal(copy, reason, err)
	}
}
