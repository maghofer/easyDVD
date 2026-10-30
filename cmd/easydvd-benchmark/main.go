// easydvd-benchmark measures the production encoder against a normalized reference.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"easydvd/internal/app"
	"easydvd/internal/media"
	"easydvd/internal/model"
	"easydvd/internal/process"
)

type Result struct {
	Run                int
	Encoder            string
	Status             string
	Seconds            float64
	CPUSeconds         float64
	CPUPercent         float64
	EncodingSeconds    float64
	EncodingCPUPercent float64
	EncodingFPS        float64
	Steps              []process.Metrics
	Bytes              int64
	PSNR               string
	SSIM               string
	DVDISO             string
	Error              string
}
type Report struct {
	InputMode       string
	LogicalCPUs     int
	Repeats         int
	Preparation     []process.Metrics
	Source          string
	SourceSHA256    string
	Reference       string
	ReferenceSHA256 string
	Standard        string
	Profile         string
	Duration        float64
	Tools           []media.ToolStatus
	Results         []Result
	Notes           []string
}

func main() {
	if ok, err := app.Worker(os.Args[1:]); ok {
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	input := flag.String("input", "", "Representative source video")
	output := flag.String("output", "", "New directory for references, encoded samples, ISOs and report")
	standard := flag.String("standard", "PAL", "PAL or NTSC")
	profile := flag.String("profile", model.ProfileBalanced, "Balanced or Quality production encoding")
	seconds := flag.Float64("seconds", 10, "Maximum sample duration")
	offset := flag.Float64("offset", 0, "Start time in seconds for the sample")
	repeats := flag.Int("repeats", 1, "Number of sequential production encoder runs")
	sourcePipeline := flag.Bool("source-pipeline", false, "Include source decoding/scaling; input must be a complete sample within -seconds, with -offset 0")
	flag.Parse()
	if *offset < 0 || *repeats < 1 {
		return fmt.Errorf("offset must be nonnegative and repeats positive")
	}
	if *input == "" || *output == "" {
		return fmt.Errorf("use -input VIDEO -output NEW_DIRECTORY")
	}
	if *seconds <= 0 {
		return fmt.Errorf("sample duration must be positive")
	}
	dir, err := filepath.Abs(*output)
	if err != nil {
		return err
	}
	if _, err = os.Stat(dir); !os.IsNotExist(err) {
		return fmt.Errorf("benchmark output must be a new directory")
	}
	if err = os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	source, err := filepath.Abs(*input)
	if err != nil {
		return err
	}
	s := model.DefaultSettings(dir)
	s.Standard = *standard
	s.EncodingProfile = *profile
	s.WorkDir = filepath.Join(dir, "work")
	s.OutputDir = filepath.Join(dir, "output")
	s.Encoders = 1
	if err = s.Validate(); err != nil {
		return err
	}
	tools := media.FindTools()
	if err = tools.Require(); err != nil {
		return err
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	log, err := os.Create(filepath.Join(dir, "benchmark.log"))
	if err != nil {
		return err
	}
	defer log.Close()
	original, err := tools.Probe(ctx, source, filepath.Base(source))
	if err != nil {
		return err
	}
	var preparation []process.Metrics
	prepCtx := process.Observe(ctx, func(m process.Metrics) { preparation = append(preparation, m) })
	original, err = tools.DetectPillarbox(prepCtx, original, source, log)
	if err != nil {
		return err
	}
	duration := min(original.Duration-*offset, *seconds)
	if duration <= 0 {
		return fmt.Errorf("offset is beyond source duration")
	}
	if *sourcePipeline && (*offset != 0 || duration < original.Duration) {
		return fmt.Errorf("-source-pipeline needs a complete sample: use -offset 0 and -seconds at least the source duration")
	}
	// Progressive references make frame-aligned PSNR/SSIM meaningful. The full
	// source pipeline benchmark deliberately excludes retained interlaced input.
	interlaced := original.FieldOrder == "tt" || original.FieldOrder == "bb" || original.FieldOrder == "tb" || original.FieldOrder == "bt"
	if *sourcePipeline && interlaced {
		return fmt.Errorf("use a progressive sample for -source-pipeline frame quality measurements")
	}
	normalized := original
	normalized.FieldOrder = "progressive"
	filter, _ := media.VideoFilter(normalized, s)
	if interlaced {
		filter = "bwdif=mode=send_frame:parity=auto:deint=all," + filter
	}

	reference := filepath.Join(dir, "reference.y4m")
	if err = process.Run(prepCtx, tools.FFmpeg, []string{"-hide_banner", "-nostdin", "-y", "-ss", fmt.Sprintf("%.6f", *offset), "-i", source, "-map", fmt.Sprintf("0:%d", original.VideoStream), "-vf", filter, "-t", fmt.Sprintf("%.6f", duration), "-pix_fmt", "yuv420p", "-an", "-f", "yuv4mpegpipe", reference}, process.Options{Log: log}); err != nil {
		return err
	}
	ref := model.Video{ID: "reference", Path: reference, Relative: "reference.y4m", Label: "Encoder sample", Width: 720, Height: s.Height(), DAR: 4.0 / 3, FPS: media.Ratio(s.FPS()), Duration: duration, AudioStream: -1, FieldOrder: "progressive", CropChecked: true, CropVersion: media.PillarboxVersion}
	if original.Aspect() == "16:9" {
		ref.DAR = 16.0 / 9
	}
	report := Report{Source: source, Reference: reference, Standard: s.Standard, Profile: s.Profile(), Duration: duration, Tools: tools.Check(ctx),
		LogicalCPUs: runtime.NumCPU(), Repeats: *repeats, Preparation: preparation,
		InputMode: "DVD-resolution prepared progressive reference",
		Notes: []string{
			fmt.Sprintf("The %s two-pass FFmpeg profile targets %d kbps video with a 9 Mbps peak and %d kbps stereo AC-3. Video sizes exclude audio and DVD muxing.", s.Profile(), s.VideoBitrate()/1000, s.AudioBitrate()/1000),
			"Seconds includes production encoding, audio/mux, complete decode validation and checkpoint hashing; EncodingSeconds isolates both MPEG-2 passes. Reference preparation, quality measurements and authoring are outside these timings.",
			"CPU percentages divide process CPU seconds by wall seconds and all logical CPUs. Runs are sequential.",
			"SSIM/PSNR compare decoded output against the DVD-resolution reference. Visual assessment and physical-player compatibility still require separate review.",
		}}
	if *sourcePipeline {
		report.InputMode = "Original-resolution progressive source decoding and scaling"
	}

	report.SourceSHA256, err = media.HashFile(ctx, source)
	if err != nil {
		return err
	}
	report.ReferenceSHA256, err = media.HashFile(ctx, reference)
	if err != nil {
		return err
	}
	failed := false
	for run := 1; run <= *repeats; run++ {
		name := "FFmpeg"

		result := Result{Run: run, Encoder: name, Status: "encoding"}
		measured := process.Observe(ctx, func(m process.Metrics) { result.Steps = append(result.Steps, m) })
		candidate := filepath.Join(dir, fmt.Sprintf("%02d-%s", run, strings.ToLower(name)))
		if err = os.MkdirAll(candidate, 0755); err != nil {
			return err
		}
		start := time.Now()
		video, input := ref, reference
		if *sourcePipeline {
			video, input = original, source
		}
		v, encodeErr := tools.Encode(measured, video, s, input, candidate, log, nil)
		err = encodeErr
		encoded := v.Encoded

		result.Seconds = time.Since(start).Seconds()
		var encodingCPU float64
		for _, step := range result.Steps {
			result.CPUSeconds += step.CPUSeconds
			if hasArg(step.Args, "-pass") {
				result.EncodingSeconds += step.Seconds
				encodingCPU += step.CPUSeconds
			}
		}
		result.CPUPercent = result.CPUSeconds / result.Seconds / float64(runtime.NumCPU()) * 100
		if result.EncodingSeconds > 0 {
			result.EncodingCPUPercent = encodingCPU / result.EncodingSeconds / float64(runtime.NumCPU()) * 100
			result.EncodingFPS = duration * media.Ratio(s.FPS()) / result.EncodingSeconds
		}
		if err != nil {
			failed = true
			result.Status = "failed"
			result.Error = err.Error()
			report.Results = append(report.Results, result)
			continue
		}
		result.Status = "encoded"
		st, e := os.Stat(encoded)
		if e != nil {
			failed = true
			result.Status = "failed"
			result.Error = e.Error()
			report.Results = append(report.Results, result)
			continue
		}
		result.Bytes = st.Size()
		elementary := filepath.Join(candidate, "video.m2v")
		err = process.Run(ctx, tools.FFmpeg, []string{"-v", "error", "-y", "-i", encoded, "-map", "0:v:0", "-c:v", "copy", "-an", "-f", "mpeg2video", elementary}, process.Options{Log: log})
		if err == nil {
			st, err = os.Stat(elementary)
			if err == nil {
				result.Bytes = st.Size()
			}
		}
		result.PSNR = metric(ctx, tools, reference, encoded, "psnr", log)
		result.SSIM = metric(ctx, tools, reference, encoded, "ssim", log)
		if err == nil && (strings.HasPrefix(result.PSNR, "failed:") || strings.HasPrefix(result.SSIM, "failed:") || result.PSNR == "unavailable" || result.SSIM == "unavailable") {
			err = fmt.Errorf("quality measurement failed: PSNR %s; SSIM %s", result.PSNR, result.SSIM)
		}
		dvdVideo := encoded
		if err == nil {
			v := ref
			v.Encoded = dvdVideo
			j := model.Job{Title: name + " sample", Settings: s, Videos: []model.Video{v}}
			iso, e := tools.Author(ctx, j, model.Disc{Number: 1, VideoIDs: []string{v.ID}}, 1, filepath.Join(candidate, "dvd-check"), log, func(string) error { return nil })
			if e != nil {
				err = e
			} else {
				result.DVDISO = iso
				result.Status = "authored"
			}
		}
		if err != nil {
			result.Error = err.Error()
			result.Status = "validation failed"
			failed = true
		}
		report.Results = append(report.Results, result)
		fmt.Printf("Run %d %s: %s · %.2fs total · %.2fs MPEG-2 · %.1f%% CPU · %d bytes · SSIM %s · PSNR %s\n", run, name, result.Status, result.Seconds, result.EncodingSeconds, result.EncodingCPUPercent, result.Bytes, result.SSIM, result.PSNR)
	}
	b, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	if err = os.WriteFile(filepath.Join(dir, "report.json"), b, 0644); err != nil {
		return err
	}
	fmt.Println("Report:", filepath.Join(dir, "report.json"))
	if failed {
		return fmt.Errorf("benchmark failed; inspect report.json and benchmark.log")
	}
	return ctx.Err()
}

func hasArg(args []string, want string) bool {
	for _, arg := range args {
		if arg == want {
			return true
		}
	}
	return false
}
func metric(ctx context.Context, t media.Tools, reference, encoded, name string, log io.Writer) string {
	var b strings.Builder
	filter := "[0:v]setpts=PTS-STARTPTS[r];[1:v]setpts=PTS-STARTPTS[d];[r][d]" + name
	err := process.Run(ctx, t.FFmpeg, []string{"-hide_banner", "-nostdin", "-i", reference, "-i", encoded, "-filter_complex", filter, "-an", "-f", "null", "-"}, process.Options{Log: io.MultiWriter(log, &b)})
	if err != nil {
		return "failed: " + err.Error()
	}
	expression := `average:([0-9.inf]+)`
	if name == "ssim" {
		expression = `All:([0-9.]+)`
	}
	match := regexp.MustCompile(expression).FindStringSubmatch(b.String())
	if len(match) > 1 {
		if _, err = strconv.ParseFloat(match[1], 64); err == nil || match[1] == "inf" {
			return match[1]
		}
	}
	return "unavailable"
}
