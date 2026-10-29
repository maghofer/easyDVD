// easydvd-benchmark compares MPEG-2 encoders against a single normalized reference.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
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
	seconds := flag.Float64("seconds", 10, "Maximum sample duration")
	offset := flag.Float64("offset", 0, "Start time in seconds for the sample")
	repeats := flag.Int("repeats", 1, "Repeated runs, alternating encoder order")
	sourcePipeline := flag.Bool("source-pipeline", false, "Include source decoding/scaling; input must be a complete sample within -seconds, with -offset 0")
	mpeg := flag.String("mpeg2enc", "", "Optional mpeg2enc executable")
	hc := flag.String("hcenc", "", "Optional HCenc executable")
	ffms := flag.String("ffms2", "", "FFMS2 AviSynth plugin DLL, required for HCenc")
	flag.Parse()
	if *offset < 0 || *repeats < 1 {
		return fmt.Errorf("offset must be nonnegative and repeats positive")
	}
	if *input == "" || *output == "" {
		return fmt.Errorf("use -input VIDEO -output NEW_DIRECTORY [-mpeg2enc EXE] [-hcenc EXE -ffms2 DLL]")
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
	s.WorkDir = filepath.Join(dir, "work")
	s.OutputDir = filepath.Join(dir, "output")
	s.Encoders = 1
	if *hc != "" {
		s.HCencDir, _ = filepath.Abs(filepath.Dir(*hc))
	}
	if *ffms != "" && *hc != "" && !strings.EqualFold(filepath.Clean(filepath.Dir(*ffms)), s.HCencDir) {
		return fmt.Errorf("use a complete private HCenc runtime with ffms2.dll beside HCenc_028.exe")
	}
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
	// Normalize once; encoder comparisons must not include different scaling or cadence conversions.
	interlaced := original.FieldOrder == "tt" || original.FieldOrder == "bb" || original.FieldOrder == "tb" || original.FieldOrder == "bt"
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
	report := Report{Source: source, Reference: reference, Standard: s.Standard, Duration: duration, Tools: tools.Check(ctx), Notes: []string{"Prepared frames are identical for all encoder inputs; audio and menu size are excluded from video-size comparisons.", "FFmpeg and HCenc target 8 Mbps average / 9 Mbps peak. mpeg2enc uses quality 2 with an 8 Mbps cap; compare measured sizes as well as quality.", "SSIM and PSNR are supporting measurements. Review grain, fine detail, motion, banding and cadence visually before choosing an encoder.", "A generated ISO confirms basic authoring interoperability; physical-player validation and full DVD/VBV conformance remain separate checks.", "HCenc requires a matching AviSynth installation and FFMS2 plugin. Optional encoders are not redistributed by easyDVD."}}
	report.LogicalCPUs, report.Repeats, report.Preparation = runtime.NumCPU(), *repeats, preparation
	report.InputMode = "DVD-resolution prepared reference"
	if *sourcePipeline {
		report.InputMode = "Original-resolution source decoding and scaling"
	}
	report.Notes = append(report.Notes, "Seconds includes the production encode path (HCenc lossless preparation, audio/mux, full decode validation and checkpoint hashing). EncodingSeconds isolates the two MPEG-2 passes. CPU percentages use process CPU seconds divided by wall seconds and all logical CPUs. Preparation is recorded separately. Encoder order alternates between repeats; no candidates run concurrently.")
	report.SourceSHA256, _ = media.HashFile(ctx, source)
	report.ReferenceSHA256, _ = media.HashFile(ctx, reference)
	for run := 1; run <= *repeats; run++ {
		names := []string{"FFmpeg", "HCenc", "mpeg2enc"}
		if run%2 == 0 {
			names = []string{"HCenc", "FFmpeg", "mpeg2enc"}
		}
		for _, name := range names {
			result := Result{Run: run, Encoder: name, Status: "not available"}
			measured := process.Observe(ctx, func(m process.Metrics) { result.Steps = append(result.Steps, m) })
			candidate := filepath.Join(dir, fmt.Sprintf("%02d-%s", run, strings.ToLower(name)))
			if err = os.MkdirAll(candidate, 0755); err != nil {
				return err
			}
			encoded := ""
			start := time.Now()
			switch name {
			case "FFmpeg", "HCenc":
				settings := s
				settings.VideoEncoder = name
				if name == "HCenc" {
					if e := tools.RequireEncoder(settings); e != nil {
						result.Error = e.Error()
						report.Results = append(report.Results, result)
						continue
					}
				}
				var v model.Video
				video, input := ref, reference
				if *sourcePipeline {
					video, input = original, source
				}
				v, err = tools.Encode(measured, video, settings, input, candidate, log, nil)
				encoded = v.Encoded
			case "mpeg2enc":
				binary := *mpeg
				if binary == "" {
					binary, _ = exec.LookPath("mpeg2enc")
				}
				if binary == "" {
					report.Results = append(report.Results, result)
					continue
				}
				binary, _ = filepath.Abs(binary)
				encoded = filepath.Join(candidate, "video.m2v")
				f, e := os.Open(reference)
				if e != nil {
					return e
				}
				norm := "p"
				if s.Standard == "NTSC" {
					norm = "n"
				}
				aspect := "2"
				if ref.Aspect() == "16:9" {
					aspect = "3"
				}
				err = process.Run(measured, binary, []string{"-f", "8", "-n", norm, "-a", aspect, "-b", "8000", "-q", "2", "-I", "0", "-o", encoded}, process.Options{Stdin: f, Log: log})
				f.Close()
			}
			result.Seconds = time.Since(start).Seconds()
			var encodingCPU float64
			for _, step := range result.Steps {
				result.CPUSeconds += step.CPUSeconds
				if strings.HasPrefix(strings.ToLower(filepath.Base(step.Executable)), "hcenc") || hasArg(step.Args, "-pass") {
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
				result.Status = "failed"
				result.Error = err.Error()
				report.Results = append(report.Results, result)
				continue
			}
			result.Status = "encoded"
			st, e := os.Stat(encoded)
			if e != nil {
				result.Status = "failed"
				result.Error = e.Error()
				report.Results = append(report.Results, result)
				continue
			}
			result.Bytes = st.Size()
			if name == "FFmpeg" || name == "HCenc" {
				elementary := filepath.Join(candidate, "video.m2v")
				err = process.Run(ctx, tools.FFmpeg, []string{"-v", "error", "-y", "-i", encoded, "-map", "0:v:0", "-c:v", "copy", "-an", "-f", "mpeg2video", elementary}, process.Options{Log: log})
				if err == nil {
					st, _ = os.Stat(elementary)
					result.Bytes = st.Size()
				}
			}
			result.PSNR = metric(ctx, tools, reference, encoded, "psnr", log)
			result.SSIM = metric(ctx, tools, reference, encoded, "ssim", log)
			dvdVideo := encoded
			if name != "FFmpeg" && name != "HCenc" {
				dvdVideo = filepath.Join(candidate, "video.mpg")
				err = process.Run(ctx, tools.FFmpeg, []string{"-hide_banner", "-nostdin", "-y", "-i", encoded, "-f", "lavfi", "-i", "anullsrc=r=48000:cl=stereo", "-map", "0:v:0", "-map", "1:a:0", "-c:v", "copy", "-c:a", "ac3", "-ac", "2", "-b:a", "256k", "-t", fmt.Sprintf("%.6f", duration), "-f", "dvd", "-muxrate", "10080k", "-packetsize", "2048", dvdVideo}, process.Options{Log: log})
			}
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
				result.Status = "authoring failed"
			}
			report.Results = append(report.Results, result)
			fmt.Printf("Run %d %s: %s · %.2fs total · %.2fs MPEG-2 · %.1f%% CPU · %d bytes · SSIM %s · PSNR %s\n", run, name, result.Status, result.Seconds, result.EncodingSeconds, result.EncodingCPUPercent, result.Bytes, result.SSIM, result.PSNR)
		}
	}
	b, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	if err = os.WriteFile(filepath.Join(dir, "report.json"), b, 0644); err != nil {
		return err
	}
	fmt.Println("Report:", filepath.Join(dir, "report.json"))
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
