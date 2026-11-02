package media

import (
	"context"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"easydvd/internal/model"
	"easydvd/internal/process"
)

func interlacedSource(v model.Video) bool {
	return v.FieldOrder == "tt" || v.FieldOrder == "bb" || v.FieldOrder == "tb" || v.FieldOrder == "bt"
}

// BlurayTarget chooses a standard raster and cadence without speeding footage up.
func BlurayTarget(v model.Video) model.BlurayPlan {
	p := model.BlurayPlan{Version: 1, Mode: "1080", Width: 1920, Height: 1080, FPSNum: 24, FPSDen: 1, Aspect: 3}
	fps := v.FPS
	if fps <= 0 {
		fps = 24
	}
	film := math.Abs(fps-24) < .05
	ntscFilm := math.Abs(fps-24000.0/1001) < .005
	pal := math.Abs(fps-25) < 1 || math.Abs(fps-50) < 2
	if v.Height <= 576 && v.Width <= 1024 {
		p.Width = 720
		if v.Height > 480 || pal {
			p.Mode, p.Height, p.FPSNum = "576", 576, 25
		} else {
			p.Mode, p.Height, p.FPSNum, p.FPSDen = "480", 480, 30000, 1001
		}
		if v.Aspect() == "4:3" {
			p.Aspect = 2
		}
	} else if v.Height <= 720 && v.Width <= 1280 {
		p.Mode, p.Width, p.Height = "720", 1280, 720
		if !film && !ntscFilm {
			if pal || fps < 27 {
				p.FPSNum = 50
			} else {
				p.FPSNum, p.FPSDen = 60000, 1001
			}
		}
	} else if !film && !ntscFilm && fps >= 24.5 {
		if pal {
			p.FPSNum = 25
		} else {
			p.FPSNum, p.FPSDen = 30000, 1001
		}
	}
	if (film || ntscFilm) && (p.Mode == "1080" || p.Mode == "720") {
		p.FPSNum, p.FPSDen = 24, 1
		if ntscFilm {
			p.FPSNum, p.FPSDen = 24000, 1001
		}
	}
	interlaceMode := p.Mode == "576" || p.Mode == "480" || p.Mode == "1080" && p.FPSNum != 24 && p.FPSNum != 24000
	p.Weave = interlaceMode && !interlacedSource(v) && fps > 40
	p.Interlaced = interlaceMode && (interlacedSource(v) || p.Weave)
	p.Channels = max(1, min(6, v.Source.AudioChannels))
	if v.AudioStream < 0 || v.Source.AudioChannels == 0 {
		p.Channels = 2
	}
	p.CopyAudio = v.AudioStream >= 0 && v.Source.AudioCodec == "ac3" && v.Source.AudioRate == 48000 && v.Source.AudioChannels <= 6 && v.Source.AudioChannels > 0 && v.Source.AudioBitrate > 0 && v.Source.AudioBitrate <= 640000
	p.AudioReason = "Convert selected audio to 48 kHz AC-3"
	// Only copy when timing is known to cover the picture. Otherwise apad is needed.
	if p.CopyAudio && (v.Source.AudioDuration <= 0 || v.Source.AudioDuration < v.Duration-.02 || math.Abs(v.Source.AudioStart-v.Source.VideoStart) > .001) {
		p.CopyAudio = false
	}
	if p.CopyAudio {
		p.AudioReason = "Copy compatible AC-3"
	}
	if v.Source.AudioChannels > 6 {
		p.AudioReason = "Downmix selected audio to 5.1 AC-3"
	}
	if v.AudioStream < 0 {
		p.AudioReason = "Create silent stereo AC-3"
	}
	return p
}

func blurayFPS(p model.BlurayPlan) string { return fmt.Sprintf("%d/%d", p.FPSNum, p.FPSDen) }

func BlurayFilter(v model.Video, p model.BlurayPlan) string {
	var filters []string
	if v.CropLeft+v.CropRight > 0 {
		filters = append(filters, fmt.Sprintf("crop=%d:%d:%d:0", v.Width-v.CropLeft-v.CropRight, v.Height, v.CropLeft))
	}
	keep := p.Interlaced && !p.Weave && interlacedSource(v) && math.Abs(v.FPS-float64(p.FPSNum)/float64(p.FPSDen)) < .02 && v.Rotation == 0
	if interlacedSource(v) && !keep {
		filters = append(filters, "bwdif=mode=send_field:parity=auto:deint=all")
	}
	dar := 16.0 / 9
	if p.Aspect == 2 {
		dar = 4.0 / 3
	}
	w, h := p.Width, p.Height
	if v.ContentDAR() > dar {
		h = max(2, int(math.Round(float64(h)*dar/v.ContentDAR()/2))*2)
	} else {
		w = max(2, int(math.Round(float64(w)*v.ContentDAR()/dar/2))*2)
	}
	if p.Interlaced {
		h = max(4, h/4*4)
	}
	inMatrix, outMatrix := "bt601", "bt709"
	if v.Height > 576 {
		inMatrix = "bt709"
	}
	if v.Source.ColorSpace != "" && v.Source.ColorSpace != "unknown" {
		inMatrix = v.Source.ColorSpace
	}
	if p.Height <= 576 {
		outMatrix = "bt601"
	}
	if primaries := v.Source.ColorPrimaries; primaries != "" && primaries != "unknown" && primaries != "bt709" && primaries != "bt470bg" && primaries != "smpte170m" {
		all := "bt709"
		if p.Mode == "576" {
			all = "bt601-6-625"
		} else if p.Mode == "480" {
			all = "bt601-6-525"
		}
		conversion := "colorspace=all=" + all + ":format=yuv420p"
		if v.Source.ColorTransfer == "" || v.Source.ColorTransfer == "unknown" {
			conversion += ":itrc=bt709"
		}
		if v.Source.ColorSpace == "" || v.Source.ColorSpace == "unknown" {
			if primaries == "bt2020" {
				conversion += ":ispace=bt2020nc"
			} else {
				conversion += ":ispace=bt709"
			}
		}
		filters = append(filters, conversion)
		inMatrix = outMatrix
	}
	interl := 0
	if keep {
		interl = 1
	}
	filters = append(filters, fmt.Sprintf("scale=%d:%d:flags=lanczos:interl=%d:in_color_matrix=%s:out_color_matrix=%s:out_range=tv", w, h, interl, inMatrix, outMatrix),
		fmt.Sprintf("pad=%d:%d:(ow-iw)/2:(oh-ih)/2:black", p.Width, p.Height),
		fmt.Sprintf("setsar=%d/%d", int(math.Round(dar*float64(p.Height))), p.Width), "format=yuv420p")
	if p.Weave || p.Interlaced && !keep {
		filters = append(filters, fmt.Sprintf("fps=%d/%d", 2*p.FPSNum, p.FPSDen), "tinterlace=mode=interleave_top")
	} else {
		filters = append(filters, "fps="+blurayFPS(p))
		if keep && (v.FieldOrder == "bb" || v.FieldOrder == "bt") {
			filters = append(filters, "fieldorder=tff")
		}
	}
	return strings.Join(filters, ",")
}

func BlurayEncodeArgs(v model.Video, s model.Settings, input, output string) []string {
	p := v.Bluray
	args := append(ffmpegInputArgs(s), "-i", input)
	if v.AudioStream < 0 {
		args = append(args, "-f", "lavfi", "-i", "anullsrc=r=48000:cl=stereo")
	}
	args = append(args, "-map", fmt.Sprintf("0:%d", v.VideoStream))
	if p.CopyVideo {
		args = append(args, "-c:v", "copy", "-bsf:v", "h264_mp4toannexb")
	} else {
		preset, crf := "medium", "20"
		if s.Profile() == model.ProfileQuality {
			preset, crf = "slow", "18"
		}
		params := fmt.Sprintf("bluray-compat=1:aud=1:slices=4:nal-hrd=vbr:force-cfr=1:keyint=%d:min-keyint=1:open-gop=0", int(math.Ceil(float64(p.FPSNum)/float64(p.FPSDen))))
		if p.Interlaced {
			params += ":interlaced=1:tff=1"
		} else if p.Mode == "576" || p.Mode == "480" || p.Mode == "1080" && p.FPSNum != 24 && p.FPSNum != 24000 {
			params += ":fake-interlaced=1:pic-struct=1"
		}
		color := "bt709"
		if p.Mode == "576" {
			color = "bt470bg"
		} else if p.Mode == "480" {
			color = "smpte170m"
		}
		params += ":colorprim=" + color + ":transfer=bt709:colormatrix=" + color + ":fullrange=off"
		args = append(args, "-vf", BlurayFilter(v, p), "-c:v", "libx264", "-preset", preset, "-crf", crf,
			"-profile:v", "high", "-level:v", "4.1", "-pix_fmt", "yuv420p", "-maxrate:v", "38000k", "-bufsize:v", "30000k", "-x264-params", params,
			"-threads:v", strconv.Itoa(encodingThreads(s)), "-r", blurayFPS(p), "-colorspace", color, "-color_primaries", color, "-color_trc", "bt709", "-color_range", "tv")
	}
	audio := fmt.Sprintf("0:%d", v.AudioStream)
	if v.AudioStream < 0 {
		audio = "1:a:0"
	}
	args = append(args, "-map", audio)
	if p.CopyAudio {
		args = append(args, "-c:a", "copy")
	} else {
		bitrate := "448k"
		if p.Channels > 2 {
			bitrate = "640k"
		} else if p.Channels == 1 {
			bitrate = "192k"
		}
		args = append(args, "-c:a", "ac3", "-ar", "48000", "-ac", strconv.Itoa(p.Channels), "-b:a", bitrate, "-af", "apad")
	}
	return append(args, "-t", fmt.Sprintf("%.6f", v.Duration), "-sn", "-dn", "-streamid", "0:4113", "-streamid", "1:4352", "-mpegts_pmt_start_pid", "256",
		"-f", "mpegts", "-mpegts_m2ts_mode", "1", "-progress", "pipe:1", "-nostats", output)
}

func (t Tools) encodeBluray(ctx context.Context, v model.Video, s model.Settings, input, dir string, log io.Writer, progress func(float64)) (model.Video, error) {
	var err error
	v, err = t.DetectPillarbox(ctx, v, input, log)
	if err != nil {
		return v, err
	}
	v.Bluray = BlurayTarget(v)
	v.Bluray.CopyVideo, v.Bluray.VideoReason, err = t.verifyBlurayAVC(ctx, v, input)
	if err != nil {
		return v, err
	}
	fmt.Fprintf(log, "Blu-ray %s: %s; %s; %s %s fps, %d audio channels\n", v.Relative, v.Bluray.VideoReason, v.Bluray.AudioReason, v.Bluray.Mode, blurayFPS(v.Bluray), v.Bluray.Channels)
	if err = os.MkdirAll(dir, 0755); err != nil {
		return v, err
	}
	part, final := filepath.Join(dir, "video.part.m2ts"), filepath.Join(dir, "video.m2ts")
	err = process.Run(ctx, t.FFmpeg, BlurayEncodeArgs(v, s, input, part), process.Options{Dir: dir, Log: log, Line: func(line string) {
		if progress != nil && strings.HasPrefix(line, "out_time_us=") {
			us, _ := strconv.ParseFloat(strings.TrimPrefix(line, "out_time_us="), 64)
			progress(math.Min(1, us/1e6/v.Duration))
		}
	}})
	if err != nil {
		return v, err
	}
	if err = process.Run(ctx, t.FFmpeg, []string{"-hide_banner", "-v", "error", "-xerror", "-i", part, "-f", "null", "-"}, process.Options{Log: log}); err != nil {
		return v, fmt.Errorf("Blu-ray media validation: %w", err)
	}
	check, err := t.Probe(ctx, part, filepath.Base(part))
	if err != nil {
		return v, err
	}
	p := v.Bluray
	if check.Source.Codec != "h264" || check.Source.PixelFormat != "yuv420p" || check.Width != p.Width || check.Height != p.Height || math.Abs(check.FPS-float64(p.FPSNum)/float64(p.FPSDen)) > .002 || check.Source.AudioCodec != "ac3" || check.Source.AudioRate != 48000 || check.Source.AudioChannels != p.Channels || math.Abs(check.Duration-v.Duration) > math.Max(.1, 2/float64(p.FPSNum)*float64(p.FPSDen)) {
		return v, fmt.Errorf("prepared Blu-ray media does not match its processing plan")
	}
	if err = os.Rename(part, final); err != nil {
		return v, err
	}
	st, err := os.Stat(final)
	if err != nil {
		return v, err
	}
	v.Encoded, v.EncodedSize, v.EncodingKey = final, st.Size(), v.ArtifactKey(s)
	v.EncodedHash, err = HashFile(ctx, final)
	return v, err
}
