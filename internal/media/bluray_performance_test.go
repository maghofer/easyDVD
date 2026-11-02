package media

import (
	"context"
	"io"
	"os"
	"testing"
	"time"

	"easydvd/internal/model"
	"easydvd/internal/process"
)

// Optional reproducible before/after benchmark using a long AVC fixture with
// unsupported reference/B-picture parameters. Accepted copies still scan EOF.
func TestBlurayInspectionEarlyRejection(t *testing.T) {
	input := os.Getenv("EASYDVD_AVC_BENCH")
	if input == "" {
		t.Skip("set EASYDVD_AVC_BENCH to an unsupported 1080p24 AVC fixture")
	}
	ff := os.Getenv("EASYDVD_AVC_BENCH_FFMPEG")
	if ff == "" {
		ff = "ffmpeg"
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	start := time.Now()
	if err := process.Run(ctx, ff, []string{"-hide_banner", "-nostdin", "-v", "info", "-xerror", "-i", input, "-map", "0:0", "-an", "-c:v", "copy", "-bsf:v", "h264_mp4toannexb,trace_headers", "-f", "null", "-"}, process.Options{Stderr: io.Discard}); err != nil {
		t.Fatal(err)
	}
	baseline := time.Since(start)
	v := model.Video{Width: 1920, Height: 1080, FPS: 24, VideoStream: 0, Source: model.StreamMetadata{Codec: "h264", PixelFormat: "yuv420p", TimeBase: "1/12288", ColorPrimaries: "bt709", ColorSpace: "bt709", ColorTransfer: "bt709", ColorRange: "tv"}}
	v.Bluray = BlurayTarget(v)
	start = time.Now()
	ok, reason, err := (Tools{FFmpeg: ff}).verifyBlurayAVC(ctx, v, input)
	optimized := time.Since(start)
	if err != nil || ok || reason == "" {
		t.Fatal(ok, reason, err)
	}
	t.Logf("full inspection=%s; early rejection=%s; speedup=%.1fx; %s", baseline, optimized, float64(baseline)/float64(optimized), reason)
}
