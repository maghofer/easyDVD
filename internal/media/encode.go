package media

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"easydvd/internal/model"
	"easydvd/internal/process"
)

func HashFile(ctx context.Context, path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	buf := make([]byte, 1024*1024)
	for {
		if err = ctx.Err(); err != nil {
			return "", err
		}
		n, e := f.Read(buf)
		if n > 0 {
			_, _ = h.Write(buf[:n])
		}
		if e == io.EOF {
			break
		}
		if e != nil {
			return "", e
		}
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func ValidArtifact(ctx context.Context, v model.Video, s model.Settings) bool {
	if v.Encoded == "" || v.EncodingKey != v.ArtifactKey(s) {
		return false
	}
	st, e := os.Stat(v.Encoded)
	if e != nil || st.Size() != v.EncodedSize {
		return false
	}
	h, e := HashFile(ctx, v.Encoded)
	return e == nil && h == v.EncodedHash
}

func VideoFilter(v model.Video, s model.Settings) (string, bool) {
	target := 4.0 / 3.0
	if v.Aspect() == "16:9" {
		target = 16.0 / 9.0
	}
	h := s.Height()
	w, scaledH := 720, h
	dar := v.ContentDAR()
	if dar > target {
		scaledH = int(math.Round(float64(h)*target/dar/2)) * 2
	} else {
		w = int(math.Round(720*dar/target/2)) * 2
	}
	if w < 2 {
		w = 2
	}
	if scaledH < 2 {
		scaledH = 2
	}
	interlaced := v.FieldOrder == "tt" || v.FieldOrder == "bb" || v.FieldOrder == "tb" || v.FieldOrder == "bt"
	matching := s.Standard == "PAL" && math.Abs(v.FPS-25) < .02 || s.Standard == "NTSC" && math.Abs(v.FPS-30000.0/1001) < .02
	keep := interlaced && matching && v.Rotation == 0
	filters := []string{}
	if v.CropLeft+v.CropRight > 0 {
		filters = append(filters, fmt.Sprintf("crop=%d:%d:%d:0", v.Width-v.CropLeft-v.CropRight, v.Height, v.CropLeft))
	}
	if interlaced && !keep {
		filters = append(filters, "bwdif=mode=send_frame:parity=auto:deint=all")
	}
	interl := "0"
	if keep {
		interl = "1"
	}
	inMatrix := "bt601"
	if v.Width >= 1280 || v.Height > 576 {
		inMatrix = "bt709"
	}
	filters = append(filters, fmt.Sprintf("scale=%d:%d:flags=lanczos:interl=%s:in_color_matrix=%s:out_color_matrix=bt601:out_range=tv", w, scaledH, interl, inMatrix), fmt.Sprintf("pad=720:%d:(ow-iw)/2:(oh-ih)/2:black", h), fmt.Sprintf("setsar=%d/%d", int(math.Round(target*float64(h))), 720), "fps="+s.FPS())
	return strings.Join(filters, ","), keep
}

func EncodeArgs(v model.Video, s model.Settings, input, passlog, output string, pass int) []string {
	args := []string{"-hide_banner", "-nostdin", "-y", "-xerror", "-err_detect", "explode", "-i", input}
	if pass == 2 && v.AudioStream < 0 {
		args = append(args, "-f", "lavfi", "-i", "anullsrc=r=48000:cl=stereo")
	}
	filter, interlaced := VideoFilter(v, s)
	flags := "+cgop"
	if interlaced {
		flags += "+ildct+ilme"
	}
	threads := max(1, (runtime.NumCPU()-2)/s.Encoders)
	if threads > 8 {
		threads = 8
	}
	// Balanced retains the old quantizer floor to avoid spending unnecessary
	// bits on easy material. Quality allows finer quantization and adds the
	// costly searches. Both keep inexpensive RD macroblock decisions.
	args = append(args, "-map", fmt.Sprintf("0:%d", v.VideoStream), "-vf", filter,
		"-c:v", "mpeg2video", "-profile:v", "4", "-level:v", "8", "-pix_fmt", "yuv420p",
		"-b:v", fmt.Sprintf("%dk", s.VideoBitrate()/1000), "-maxrate:v", "9000k", "-minrate:v", "0", "-bufsize:v", "1835008",
		"-mbd", "rd", "-intra_vlc", "1")
	if s.Profile() == model.ProfileQuality {
		args = append(args, "-qmin", "1", "-lmin", "118", "-trellis", "1", "-cmp", "satd", "-subcmp", "satd", "-mepre", "1", "-precmp", "satd",
			"-dia_size", "2", "-pre_dia_size", "2", "-last_pred", "3", "-bidir_refine", "2", "-mpv_flags", "+mv0+cbp_rd")
	} else {
		args = append(args, "-qmin", "2", "-lmin", "236")
	}
	args = append(args,
		"-g", s.GOP(), "-bf", "2", "-flags", flags, "-sc_threshold", "1000000000", "-dc", "10",
		"-aspect", v.Aspect(), "-threads", strconv.Itoa(threads), "-pass", strconv.Itoa(pass), "-passlogfile", passlog,
		"-sn", "-dn", "-progress", "pipe:1", "-nostats")
	if interlaced {
		order := "tt"
		if v.FieldOrder == "bb" || v.FieldOrder == "bt" {
			order = "bb"
		}
		args = append(args, "-field_order", order)
	}
	color := "bt470bg"
	if s.Standard == "NTSC" {
		color = "smpte170m"
	}
	args = append(args, "-colorspace", color, "-color_primaries", color, "-color_trc", "bt709")
	if pass == 1 {
		args = append(args, "-an", "-f", "null", output)
	} else {
		audio := fmt.Sprintf("0:%d", v.AudioStream)
		if v.AudioStream < 0 {
			audio = "1:a:0"
		}
		args = append(args, "-map", audio, "-c:a", "ac3", "-ar", "48000", "-ac", "2", "-b:a", fmt.Sprintf("%dk", s.AudioBitrate()/1000), "-af", "apad", "-shortest", "-t", fmt.Sprintf("%.6f", v.Duration), "-f", "dvd", "-muxrate", "10080k", "-packetsize", "2048", output)
	}
	return args
}

func (t Tools) Encode(ctx context.Context, v model.Video, s model.Settings, input, dir string, log io.Writer, progress func(float64)) (model.Video, error) {
	var err error
	v, err = t.DetectPillarbox(ctx, v, input, log)
	if err != nil {
		return v, err
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return v, err
	}
	passlog := filepath.Join(dir, "pass")
	part := filepath.Join(dir, "video.part.mpg")
	final := filepath.Join(dir, "video.mpg")
	null := "/dev/null"
	if runtime.GOOS == "windows" {
		null = "NUL"
	}
	for pass := 1; pass <= 2; pass++ {
		out := null
		if pass == 2 {
			out = part
		}
		p := pass
		err := process.Run(ctx, t.FFmpeg, EncodeArgs(v, s, input, passlog, out, pass), process.Options{Dir: dir, Log: log, Line: func(line string) {
			if strings.HasPrefix(line, "out_time_us=") {
				us, _ := strconv.ParseFloat(strings.TrimPrefix(line, "out_time_us="), 64)
				if progress != nil {
					progress(math.Min(1, (float64(p-1)+us/1e6/v.Duration)/2))
				}
			}
		}})
		if err != nil {
			return v, err
		}
	}
	// A complete decode catches truncated streams before they become reusable checkpoints.
	if err := process.Run(ctx, t.FFmpeg, []string{"-v", "error", "-xerror", "-i", part, "-f", "null", null}, process.Options{Log: log}); err != nil {
		return v, fmt.Errorf("encoded video validation failed: %w", err)
	}
	if err := os.Rename(part, final); err != nil {
		return v, err
	}
	st, err := os.Stat(final)
	if err != nil {
		return v, err
	}
	v.Encoded = final
	v.EncodedSize = st.Size()
	v.EncodingKey = v.ArtifactKey(s)
	v.EncodedHash, err = HashFile(ctx, final)
	return v, err
}

func (t Tools) Thumbnail(ctx context.Context, v model.Video, input, path string, log io.Writer) error {
	seconds := math.Min(v.Duration*.25, 30)
	w, h := 320, 240
	dar := v.ContentDAR()
	if dar >= 4.0/3 {
		h = max(2, int(math.Round(320/dar/2))*2)
	} else {
		w = max(2, int(math.Round(240*dar/2))*2)
	}
	filter := fmt.Sprintf("scale=%d:%d:flags=lanczos,pad=320:240:(ow-iw)/2:(oh-ih)/2,setsar=1", w, h)
	if v.CropLeft+v.CropRight > 0 {
		filter = fmt.Sprintf("crop=%d:%d:%d:0,", v.Width-v.CropLeft-v.CropRight, v.Height, v.CropLeft) + filter
	}
	return process.Run(ctx, t.FFmpeg, []string{"-hide_banner", "-nostdin", "-y", "-ss", fmt.Sprintf("%.3f", seconds), "-i", input, "-map", fmt.Sprintf("0:%d", v.VideoStream), "-vf", filter, "-frames:v", "1", "-update", "1", path}, process.Options{Log: log})
}
