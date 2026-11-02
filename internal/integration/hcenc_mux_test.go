package integration

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"easydvd/internal/media"
	"easydvd/internal/model"
	"easydvd/internal/process"
)

// Exercise the production muxer independently of HCEnc's expensive analysis.
// Reordered MPEG-2 pictures, exact packet payloads and decoded frame counts
// catch loss that successful decoding and approximate duration checks miss.
func TestHCencMuxPreservesFrames(t *testing.T) {
	tools, ctx := issueTools(t)
	digest := func(t *testing.T, path string) string {
		t.Helper()
		var out bytes.Buffer
		if err := process.Run(ctx, tools.FFmpeg, []string{"-v", "error", "-nostdin", "-i", path, "-map", "0:v:0", "-c:v", "copy", "-f", "hash", "-hash", "sha256", "pipe:1"}, process.Options{Stdout: &out}); err != nil {
			t.Fatal(err)
		}
		return strings.TrimSpace(out.String())
	}
	for _, standard := range []string{"PAL", "NTSC"} {
		for _, frames := range []int{1, 2, 17, 53, 300} {
			t.Run(fmt.Sprintf("%s/%d", standard, frames), func(t *testing.T) {
				dir := t.TempDir()
				s := model.DefaultSettings(dir)
				s.Standard, s.EncodingProfile, s.Encoders = standard, model.ProfileQuality, 1
				video := filepath.Join(dir, "video.m2v")
				args := []string{"-v", "error", "-nostdin", "-y", "-f", "lavfi", "-i", fmt.Sprintf("testsrc2=s=720x%d:r=%s", s.Height(), s.FPS()),
					"-frames:v", strconv.Itoa(frames), "-c:v", "mpeg2video", "-g", s.GOP(), "-bf", "2", "-aspect", "4:3", "-f", "mpeg2video", video}
				if err := process.Run(ctx, tools.FFmpeg, args, process.Options{}); err != nil {
					t.Fatal(err)
				}
				original := digest(t, video)
				audioCases := []string{"silence"}
				if frames == 53 {
					audioCases = append(audioCases, "short", "long", "selected")
				}
				for _, audio := range audioCases {
					t.Run(audio, func(t *testing.T) {
						v := model.Video{Width: 720, Height: s.Height(), FPS: media.Ratio(s.FPS()), Duration: float64(frames) / media.Ratio(s.FPS()), DAR: 4.0 / 3, AudioStream: -1}
						input := filepath.Join(dir, audio+".mka")
						if audio != "silence" {
							duration := v.Duration + 1
							if audio == "short" {
								duration = .25
							}
							audioArgs := []string{"-v", "error", "-nostdin", "-y"}
							if audio == "selected" {
								audioArgs = append(audioArgs, "-f", "lavfi", "-i", fmt.Sprintf("anullsrc=r=48000:cl=stereo:d=%.6f", duration))
							}
							audioArgs = append(audioArgs, "-f", "lavfi", "-i", fmt.Sprintf("sine=frequency=880:sample_rate=48000:duration=%.6f", duration), "-map", "0:a:0")
							if audio == "selected" {
								audioArgs = append(audioArgs, "-map", "1:a:0")
							}
							audioArgs = append(audioArgs, "-c:a", "pcm_s16le", input)
							if err := process.Run(ctx, tools.FFmpeg, audioArgs, process.Options{}); err != nil {
								t.Fatal(err)
							}
							v.AudioStream = 0
							if audio == "selected" {
								v.AudioStream = 1
							}
						}
						output := filepath.Join(dir, audio+".mpg")
						if err := process.Run(ctx, tools.FFmpeg, media.HCencMuxArgs(v, s, input, video, output), process.Options{}); err != nil {
							t.Fatal(err)
						}
						if got := digest(t, output); got != original {
							t.Fatal("mux changed or lost encoded video payload", original, got)
						}
						var probe bytes.Buffer
						if err := process.Run(ctx, tools.FFprobe, []string{"-v", "error", "-count_frames", "-show_streams", "-of", "json", output}, process.Options{Stdout: &probe}); err != nil {
							t.Fatal(err)
						}
						var p struct {
							Streams []struct {
								Type   string `json:"codec_type"`
								Codec  string `json:"codec_name"`
								Frames string `json:"nb_read_frames"`
								Start  string `json:"start_time"`
							} `json:"streams"`
						}
						if err := json.Unmarshal(probe.Bytes(), &p); err != nil {
							t.Fatal(err)
						}
						videoStart, audioStart, videoEnd, audioEnd := math.NaN(), math.NaN(), math.NaN(), math.NaN()
						for _, stream := range p.Streams {
							start := media.Ratio(stream.Start)
							switch stream.Type {
							case "video":
								if stream.Frames != strconv.Itoa(frames) || stream.Codec != "mpeg2video" {
									t.Fatal("decoded video frame count changed", probe.String())
								}
								videoStart, videoEnd = start, start+v.Duration
							case "audio":
								if stream.Codec != "ac3" {
									t.Fatal("expected AC-3 audio", probe.String())
								}
								packets, err := strconv.Atoi(stream.Frames)
								if err != nil || packets < 1 {
									t.Fatal("no decoded audio frames", probe.String())
								}
								// Stream duration estimates are unreliable on tiny MPEG
								// inputs; decoded AC-3 frames each contain 1536 samples.
								audioStart, audioEnd = start, start+float64(packets)*1536/48000
							}
						}
						// AC-3 encoder delay is 256 samples (5.33 ms); its packets
						// span 1536 samples (32 ms), so the end must fit one packet.
						if math.IsNaN(videoStart+audioStart+videoEnd+audioEnd) || math.Abs(videoStart-audioStart) > .006 || math.Abs(videoEnd-audioEnd) > .033 {
							t.Fatal("audio/video alignment drift", probe.String())
						}
						if audio == "selected" {
							var pcm bytes.Buffer
							if err := process.Run(ctx, tools.FFmpeg, []string{"-v", "error", "-nostdin", "-i", output, "-map", "0:a:0", "-t", "0.1", "-c:a", "pcm_s16le", "-f", "s16le", "pipe:1"}, process.Options{Stdout: &pcm}); err != nil {
								t.Fatal(err)
							}
							peak := 0
							for b := pcm.Bytes(); len(b) >= 2; b = b[2:] {
								peak = max(peak, int(math.Abs(float64(int16(binary.LittleEndian.Uint16(b))))))
							}
							if peak < 1000 {
								t.Fatal("mux selected the silent first audio stream", peak)
							}
						}
					})
				}
			})
		}
	}
}
