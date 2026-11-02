package media

import (
	"fmt"
	"strings"

	"easydvd/internal/model"
	"easydvd/internal/process"
)

// EncodingCommands uses the production argument builders, so diagnostics track
// profile changes. Paths are placeholders; each job log records resolved runs.
func (t Tools) EncodingCommands(s model.Settings) string {
	var b strings.Builder
	b.WriteString("ENCODING COMMANDS\nTemplates for a progressive 16:9 title with stereo audio (120 seconds). Stream indices, duration, crop, field order, silence, thread budget and Blu-ray target vary per title. Resolved invocations are recorded in each job log. Workers are application-wide.\n\n")
	ff := t.FFmpeg
	if ff == "" {
		ff = "ffmpeg"
	}
	command := func(exe string, args []string) {
		b.WriteString(process.CommandLine(exe, args))
		b.WriteString("\n\n")
	}
	v := model.Video{Width: 1920, Height: 1080, DAR: 16.0 / 9, FPS: 24, Duration: 120, VideoStream: 0, AudioStream: 1, Source: model.StreamMetadata{AudioChannels: 2}}
	for _, standard := range []string{"PAL", "NTSC"} {
		s.DiscFormat = "DVD"
		s.Standard = standard
		s.EncodingProfile = model.ProfileBalanced
		fmt.Fprintf(&b, "DVD %s — Balanced (two passes)\n", standard)
		command(ff, EncodeArgs(v, s, "<source>", "<work>/pass", "NUL", 1))
		command(ff, EncodeArgs(v, s, "<source>", "<work>/pass", "<work>/video.part.mpg", 2))
		s.EncodingProfile = model.ProfileQuality
		fmt.Fprintf(&b, "DVD %s — Quality (HCEnc, two passes)\n", standard)
		command(ff, HCencPrepareArgs(v, s, "<source>", "<work>/prepared.avi"))
		command("<private HCEnc>/HCenc.exe", []string{"-ini", "<work>/hcenc.ini", "-2pass", "-silent", "-wait", "0"})
		b.WriteString("hcenc.ini (progressive; interlaced sources use INTERLACED and TFF/BFF):\n" + HCencINI(v, s, false) + "\n")
		command(ff, HCencMuxArgs(v, s, "<source>", "<work>/video.m2v", "<work>/video.part.mpg"))
	}
	s.DiscFormat = "Blu-ray"
	v.Bluray = BlurayTarget(v)
	for _, profile := range []string{model.ProfileBalanced, model.ProfileQuality} {
		s.EncodingProfile = profile
		fmt.Fprintf(&b, "Blu-ray — %s (1080p24 example)\n", profile)
		command(ff, BlurayEncodeArgs(v, s, "<source>", "<work>/video.part.m2ts"))
	}
	v.Bluray.CopyVideo = true
	v.Bluray.CopyAudio = true
	b.WriteString("Blu-ray — verified AVC / AC-3 copy (either profile)\n")
	command(ff, BlurayEncodeArgs(v, s, "<source>", "<work>/video.part.m2ts"))
	b.WriteString("Complete decode validation after DVD preparation (both profiles)\n")
	command(ff, []string{"-v", "error", "-xerror", "-i", "<work>/video.part.mpg", "-f", "null", "NUL"})
	b.WriteString("Complete decode validation after Blu-ray preparation (both profiles)\n")
	command(ff, []string{"-hide_banner", "-v", "error", "-xerror", "-i", "<work>/video.part.m2ts", "-f", "null", "-"})
	b.WriteString("Blu-ray full-stream AVC compatibility inspection (both profiles; stops on rejection)\n")
	command(ff, []string{"-hide_banner", "-nostdin", "-v", "info", "-xerror", "-i", "<source>", "-map", "0:0", "-an", "-c:v", "copy", "-bsf:v", "h264_mp4toannexb,trace_headers", "-f", "null", "-"})
	return b.String()
}
