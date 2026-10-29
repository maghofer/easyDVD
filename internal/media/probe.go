package media

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"easydvd/internal/fsops"
	"easydvd/internal/model"
	"easydvd/internal/process"
)

var extensions = map[string]bool{}

func init() {
	for _, s := range strings.Fields(".mp4 .mov .mkv .avi .mpg .mpeg .m2v .m4v .mts .m2ts .ts .vob .wmv .webm .dv .flv .3gp .mxf .ogv .mpv .divx .asf .m2t") {
		extensions[s] = true
	}
}
func IsVideo(path string) bool {
	// macOS AppleDouble files carry resource metadata, never video data.
	return !strings.HasPrefix(filepath.Base(path), "._") && extensions[strings.ToLower(filepath.Ext(path))]
}

type stream struct {
	Index       int    `json:"index"`
	Type        string `json:"codec_type"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	DAR         string `json:"display_aspect_ratio"`
	SAR         string `json:"sample_aspect_ratio"`
	FPS         string `json:"avg_frame_rate"`
	Field       string `json:"field_order"`
	Duration    string `json:"duration"`
	Transfer    string `json:"color_transfer"`
	Disposition struct {
		Default  int `json:"default"`
		Attached int `json:"attached_pic"`
	} `json:"disposition"`
	Side []struct {
		Rotation int `json:"rotation"`
	} `json:"side_data_list"`
	Tags map[string]string `json:"tags"`
}

func Ratio(s string) float64 {
	parts := strings.FieldsFunc(s, func(r rune) bool { return r == ':' || r == '/' })
	if len(parts) == 2 {
		a, _ := strconv.ParseFloat(parts[0], 64)
		b, _ := strconv.ParseFloat(parts[1], 64)
		if b != 0 {
			return a / b
		}
		return 0
	}
	f, _ := strconv.ParseFloat(s, 64)
	return f
}
func (t Tools) Probe(ctx context.Context, path, relative string) (model.Video, error) {
	v := model.Video{ID: model.Hash(relative)[:20], Path: path, Relative: relative, Label: strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)), AudioStream: -1}
	st, err := fsops.Stat(ctx, path)
	if err != nil {
		return v, err
	}
	if !st.Exists || st.Directory {
		return v, fmt.Errorf("video source is missing or is not a file")
	}
	v.Size = st.Size
	v.ModTime = st.ModTime
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	var b bytes.Buffer
	if t.FFprobe == "" {
		return v, fmt.Errorf("ffprobe is missing")
	}
	err = process.Run(ctx, t.FFprobe, []string{"-v", "error", "-show_streams", "-show_format", "-of", "json", path}, process.Options{Stdout: &b})
	if err != nil {
		return v, err
	}
	var p struct {
		Streams []stream `json:"streams"`
		Format  struct {
			Duration string `json:"duration"`
		} `json:"format"`
	}
	if err = json.Unmarshal(b.Bytes(), &p); err != nil {
		return v, err
	}
	var video *stream
	for i := range p.Streams {
		s := &p.Streams[i]
		if s.Type == "video" && s.Disposition.Attached == 0 && (video == nil || s.Disposition.Default > video.Disposition.Default) {
			video = s
		}
	}
	if video == nil {
		return v, fmt.Errorf("no decodable video stream")
	}
	if video.Transfer == "smpte2084" || video.Transfer == "arib-std-b67" {
		return v, fmt.Errorf("HDR footage requires an SDR conversion before DVD authoring")
	}
	v.VideoStream = video.Index
	v.Width = video.Width
	v.Height = video.Height
	v.DAR = Ratio(video.DAR)
	v.FPS = Ratio(video.FPS)
	v.FieldOrder = video.Field
	if v.Width <= 0 || v.Height <= 0 {
		return v, fmt.Errorf("video dimensions are unavailable")
	}
	if v.DAR <= 0 {
		sar := Ratio(video.SAR)
		if sar <= 0 {
			sar = 1
		}
		v.DAR = float64(v.Width) / float64(v.Height) * sar
	}
	for _, side := range video.Side {
		v.Rotation = side.Rotation
	}
	if r, ok := video.Tags["rotate"]; ok {
		v.Rotation, _ = strconv.Atoi(r)
	}
	if int(math.Abs(float64(v.Rotation)))%180 == 90 {
		v.DAR = 1 / v.DAR
		v.Width, v.Height = v.Height, v.Width
	}
	v.Duration = Ratio(video.Duration)
	if v.Duration <= 0 {
		v.Duration = Ratio(p.Format.Duration)
	}
	if v.Duration <= 0 || math.IsInf(v.Duration, 0) || math.IsNaN(v.Duration) {
		return v, fmt.Errorf("video duration is unavailable")
	}
	var audio *stream
	for i := range p.Streams {
		s := &p.Streams[i]
		if s.Type == "audio" && (audio == nil || s.Disposition.Default > audio.Disposition.Default) {
			audio = s
		}
	}
	if audio != nil {
		v.AudioStream = audio.Index
	}
	return v, nil
}
