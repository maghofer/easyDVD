package model

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

type State string

const (
	Scanning State = "Scanning"
	Ready    State = "Ready"
	Running  State = "Running"
	Paused   State = "Paused"
	Blocked  State = "Needs attention"
	Canceled State = "Canceled"
	Complete State = "Complete"
)

type Settings struct {
	Standard      string    `json:"standard"`
	Media         string    `json:"media"`
	Packing       string    `json:"packing"`
	Layout        string    `json:"layout"`
	TitlesPerPage int       `json:"titles_per_page"`
	StageLocally  bool      `json:"stage_locally"`
	Encoders      int       `json:"encoders"`
	VideoEncoder  string    `json:"video_encoder,omitempty"`
	HCencDir      string    `json:"hcenc_dir,omitempty"`
	OutputDir     string    `json:"output_dir"`
	WorkDir       string    `json:"work_dir"`
	Theme         string    `json:"theme"`
	KeepWork      bool      `json:"keep_work"`
	NoMenu        bool      `json:"no_menu,omitempty"`
	Menu          MenuStyle `json:"menu"`
}

func DefaultSettings(base string) Settings {
	return Settings{Standard: "PAL", Media: "DVD-5", Packing: "Strict order", Layout: "Text", TitlesPerPage: 10, StageLocally: true, Encoders: 2, VideoEncoder: "HCenc", OutputDir: filepath.Join(base, "easyDVD Output"), WorkDir: filepath.Join(base, "easyDVD Work"), Theme: "System", Menu: DefaultMenuStyle()}
}

func (s Settings) Validate() error {
	if s.Encoder() != "HCenc" && s.Encoder() != "FFmpeg" {
		return fmt.Errorf("choose HCenc or FFmpeg as video encoder")
	}
	if s.HCencDir != "" && !filepath.IsAbs(s.HCencDir) {
		return fmt.Errorf("HCenc folder must be an absolute path")
	}
	if s.Standard != "PAL" && s.Standard != "NTSC" {
		return fmt.Errorf("choose PAL or NTSC")
	}
	if s.Media != "DVD-5" && s.Media != "DVD-9" {
		return fmt.Errorf("choose DVD-5 or DVD-9")
	}
	if s.Packing != "Strict order" && s.Packing != "Space efficiency" {
		return fmt.Errorf("invalid packing mode")
	}
	if s.Layout != "Text" && s.Layout != "Thumbnails" {
		return fmt.Errorf("invalid menu layout")
	}
	max := 12
	if s.Layout == "Thumbnails" {
		max = 6
	}
	if s.TitlesPerPage < 1 || s.TitlesPerPage > max {
		return fmt.Errorf("titles per page must be 1–%d for this layout", max)
	}
	if s.Encoders < 1 || s.Encoders > s.MaxEncoders() {
		return fmt.Errorf("use 1–%d encoding workers with %s", s.MaxEncoders(), s.Encoder())
	}
	if !filepath.IsAbs(s.OutputDir) || !filepath.IsAbs(s.WorkDir) {
		return fmt.Errorf("output and work folders must be absolute paths")
	}
	if filepath.Clean(s.OutputDir) == filepath.Clean(s.WorkDir) {
		return fmt.Errorf("output and work folders must be different")
	}
	return s.Menu.Validate()
}

// Conservatively below the nominal writable sector counts. ISO validation is authoritative.
func (s Settings) Capacity() int64 {
	if s.Media == "DVD-9" {
		return 8540000000
	}
	return 4700000000
}
func (s Settings) Height() int {
	if s.Standard == "NTSC" {
		return 480
	}
	return 576
}
func (s Settings) FPS() string {
	if s.Standard == "NTSC" {
		return "30000/1001"
	}
	return "25"
}
func (s Settings) GOP() string {
	if s.Standard == "NTSC" {
		return "18"
	}
	return "15"
}

// Only settings affecting an encoded title belong here. Menu edits and packing do not re-encode.
func (s Settings) EncodingKey() string {
	if s.VideoEncoder == "" {
		return Hash("mpeg2-v2-8000-9000-ac3-256-2pass-" + s.Standard)
	}
	return Hash("mpeg2-v4-pillarbox-bounds-8000-9000-ac3-256-2pass-" + s.Encoder() + "-" + s.Standard)
}

// Empty identifies queues saved before encoder selection existed.
func (s Settings) Encoder() string {
	if s.VideoEncoder == "" {
		return "FFmpeg"
	}
	return s.VideoEncoder
}

func (s Settings) MaxEncoders() int {
	if s.Encoder() == "HCenc" {
		return 4
	}
	return 2
}

func (v Video) ArtifactKey(s Settings) string {
	key := s.EncodingKey() + v.Fingerprint()
	if v.CropLeft+v.CropRight > 0 {
		key += fmt.Sprintf(":crop:%d:%d", v.CropLeft, v.CropRight)
	}
	return key
}

type Video struct {
	ID          string  `json:"id"`
	Path        string  `json:"path"`
	Relative    string  `json:"relative"`
	Label       string  `json:"label"`
	Size        int64   `json:"size"`
	ModTime     int64   `json:"mtime"`
	Duration    float64 `json:"duration"`
	Width       int     `json:"width"`
	Height      int     `json:"height"`
	DAR         float64 `json:"dar"`
	CropLeft    int     `json:"crop_left,omitempty"`
	CropRight   int     `json:"crop_right,omitempty"`
	CropChecked bool    `json:"crop_checked,omitempty"`
	CropVersion int     `json:"crop_version,omitempty"`
	FPS         float64 `json:"fps"`
	FieldOrder  string  `json:"field_order"`
	VideoStream int     `json:"video_stream"`
	AudioStream int     `json:"audio_stream"`
	Rotation    int     `json:"rotation"`
	Excluded    bool    `json:"excluded"`
	Error       string  `json:"error,omitempty"`
	Encoded     string  `json:"encoded,omitempty"`
	EncodedSize int64   `json:"encoded_size"`
	EncodedHash string  `json:"encoded_hash,omitempty"`
	EncodingKey string  `json:"encoding_key,omitempty"`
	Thumbnail   string  `json:"thumbnail,omitempty"`
}

// OriginalLabel matches the title initially assigned by the media probe.
func (v Video) OriginalLabel() string {
	path := v.Path
	if path == "" {
		path = v.Relative
	}
	if path == "" {
		return v.Label
	}
	return strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
}

func (v Video) Fingerprint() string   { return Hash(fmt.Sprintf("%s:%d:%d", v.Path, v.Size, v.ModTime)) }
func (v Video) EstimatedBytes() int64 { return int64(v.Duration * 8256000 / 8 * 1.035) }
func (v Video) Aspect() string {
	if v.ContentDAR() >= 1.55 {
		return "16:9"
	}
	return "4:3"
}

func (v Video) ContentDAR() float64 {
	if v.Width > 0 && v.CropLeft >= 0 && v.CropRight >= 0 && v.CropLeft+v.CropRight < v.Width {
		return v.DAR * float64(v.Width-v.CropLeft-v.CropRight) / float64(v.Width)
	}
	return v.DAR
}

type Disc struct {
	Number   int      `json:"number"`
	VideoIDs []string `json:"video_ids"`
	Bytes    int64    `json:"bytes"`
	ISO      string   `json:"iso,omitempty"`
	SHA256   string   `json:"sha256,omitempty"`
	ISOBytes int64    `json:"iso_bytes,omitempty"`
}

type Job struct {
	ID             string     `json:"id"`
	Folder         string     `json:"folder"`
	Sources        []string   `json:"sources,omitempty"`
	Title          string     `json:"title"`
	State          State      `json:"state"`
	Stage          string     `json:"stage"`
	Error          string     `json:"error,omitempty"`
	Videos         []Video    `json:"videos"`
	Discs          []Disc     `json:"discs"`
	Settings       Settings   `json:"settings"`
	Locked         bool       `json:"locked"`
	ScanComplete   bool       `json:"scan_complete"`
	Discovered     int        `json:"discovered"`
	Generation     string     `json:"generation,omitempty"`
	EstimatedDiscs int        `json:"estimated_discs"`
	MinimumProven  bool       `json:"minimum_proven"`
	Progress       float64    `json:"progress"`
	Created        time.Time  `json:"created"`
	Finished       *time.Time `json:"finished,omitempty"`
	OutputPath     string     `json:"output_path,omitempty"`
}

func (j Job) ActiveVideos() []Video {
	var out []Video
	for _, v := range j.Videos {
		if !v.Excluded {
			out = append(out, v)
		}
	}
	return out
}
func (j Job) Duration() float64 {
	var d float64
	for _, v := range j.Videos {
		if !v.Excluded {
			d += v.Duration
		}
	}
	return d
}
func (j Job) Clone() Job {
	c := j
	c.Sources = append([]string(nil), j.Sources...)
	c.Videos = append([]Video(nil), j.Videos...)
	c.Discs = append([]Disc(nil), j.Discs...)
	for i := range c.Discs {
		c.Discs[i].VideoIDs = append([]string(nil), j.Discs[i].VideoIDs...)
	}
	if j.Finished != nil {
		finished := *j.Finished
		c.Finished = &finished
	}
	return c
}
func ID() string           { var b [12]byte; _, _ = rand.Read(b[:]); return hex.EncodeToString(b[:]) }
func Hash(s string) string { b := sha256.Sum256([]byte(s)); return hex.EncodeToString(b[:]) }
func SafeName(s string) string {
	s = strings.Map(func(r rune) rune {
		if r < 32 || strings.ContainsRune(`<>:"/\|?*`, r) {
			return '_'
		}
		return r
	}, s)
	s = strings.Trim(s, " .")
	if s == "" {
		s = "DVD Set"
	}
	r := []rune(s)
	if len(r) > 80 {
		s = string(r[:80])
	}
	return s
}
func DurationString(sec float64) string {
	d := int(sec)
	return fmt.Sprintf("%02d:%02d:%02d", d/3600, d/60%60, d%60)
}
