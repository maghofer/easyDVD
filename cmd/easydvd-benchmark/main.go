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
	"strconv"
	"strings"
	"time"

	"easydvd/internal/app"
	"easydvd/internal/media"
	"easydvd/internal/model"
	"easydvd/internal/process"
)

type Result struct {
	Encoder string
	Status  string
	Seconds float64
	Bytes   int64
	PSNR    string
	SSIM    string
	DVDISO  string
	Error   string
}
type Report struct {
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
	mpeg := flag.String("mpeg2enc", "", "Optional mpeg2enc executable")
	hc := flag.String("hcenc", "", "Optional HCenc executable")
	ffms := flag.String("ffms2", "", "FFMS2 AviSynth plugin DLL, required for HCenc")
	flag.Parse()
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
	duration := min(original.Duration, *seconds)
	// Normalize once; encoder comparisons must not include different scaling or cadence conversions.
	interlaced := original.FieldOrder == "tt" || original.FieldOrder == "bb" || original.FieldOrder == "tb" || original.FieldOrder == "bt"
	normalized := original
	normalized.FieldOrder = "progressive"
	filter, _ := media.VideoFilter(normalized, s)
	if interlaced {
		filter = "bwdif=mode=send_frame:parity=auto:deint=all," + filter
	}
	reference := filepath.Join(dir, "reference.y4m")
	if err = process.Run(ctx, tools.FFmpeg, []string{"-hide_banner", "-nostdin", "-y", "-i", source, "-map", fmt.Sprintf("0:%d", original.VideoStream), "-vf", filter, "-t", fmt.Sprintf("%.6f", duration), "-pix_fmt", "yuv420p", "-an", "-f", "yuv4mpegpipe", reference}, process.Options{Log: log}); err != nil {
		return err
	}
	ref := model.Video{ID: "reference", Path: reference, Relative: "reference.y4m", Label: "Encoder sample", Width: 720, Height: s.Height(), DAR: 4.0 / 3, FPS: media.Ratio(s.FPS()), Duration: duration, AudioStream: -1, FieldOrder: "progressive"}
	if original.Aspect() == "16:9" {
		ref.DAR = 16.0 / 9
	}
	report := Report{Source: source, Reference: reference, Standard: s.Standard, Duration: duration, Tools: tools.Check(ctx), Notes: []string{"Prepared frames are identical for all encoder inputs; audio and menu size are excluded from video-size comparisons.", "FFmpeg and HCenc target 8 Mbps average / 9 Mbps peak. mpeg2enc uses quality 2 with an 8 Mbps cap; compare measured sizes as well as quality.", "SSIM and PSNR are supporting measurements. Review grain, fine detail, motion, banding and cadence visually before choosing an encoder.", "A generated ISO confirms basic authoring interoperability; physical-player validation and full DVD/VBV conformance remain separate checks.", "HCenc requires a matching AviSynth installation and FFMS2 plugin. Optional encoders are not redistributed by easyDVD."}}
	report.SourceSHA256, _ = media.HashFile(ctx, source)
	report.ReferenceSHA256, _ = media.HashFile(ctx, reference)
	for _, name := range []string{"FFmpeg", "mpeg2enc", "HCenc"} {
		result := Result{Encoder: name, Status: "not available"}
		candidate := filepath.Join(dir, strings.ToLower(name))
		if err = os.MkdirAll(candidate, 0755); err != nil {
			return err
		}
		encoded := ""
		start := time.Now()
		switch name {
		case "FFmpeg":
			var v model.Video
			v, err = tools.Encode(ctx, ref, s, reference, candidate, log, nil)
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
			err = process.Run(ctx, binary, []string{"-f", "8", "-n", norm, "-a", aspect, "-b", "8000", "-q", "2", "-I", "0", "-o", encoded}, process.Options{Stdin: f, Log: log})
			f.Close()
		case "HCenc":
			if *hc == "" || *ffms == "" {
				result.Error = "Supply -hcenc and -ffms2; install matching AviSynth separately"
				report.Results = append(report.Results, result)
				continue
			}
			binary, _ := filepath.Abs(*hc)
			plugin, _ := filepath.Abs(*ffms)
			script := fmt.Sprintf("LoadPlugin(\"%s\")\nFFVideoSource(\"%s\", cache=false)\nConvertToYV12(interlaced=false)\n", plugin, reference)
			if err = os.WriteFile(filepath.Join(candidate, "source.avs"), []byte(script), 0644); err != nil {
				return err
			}
			ini := fmt.Sprintf("*INFILE %s\n*OUTFILE %s\n*BITRATE 8000\n*MAXBITRATE 9000\n*PROFILE BEST\n*ASPECT %s\n*PROGRESSIVE\n", filepath.Join(candidate, "source.avs"), filepath.Join(candidate, "video.m2v"), ref.Aspect())
			if err = os.WriteFile(filepath.Join(candidate, "HC.ini"), []byte(ini), 0644); err != nil {
				return err
			}
			encoded = filepath.Join(candidate, "video.m2v")
			err = process.Run(ctx, binary, []string{"-ini", filepath.Join(candidate, "HC.ini")}, process.Options{Dir: candidate, Log: log})
		}
		result.Seconds = time.Since(start).Seconds()
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
		if name == "FFmpeg" {
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
		if name != "FFmpeg" {
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
		fmt.Printf("%s: %s · %.2fs · %d bytes · SSIM %s · PSNR %s\n", name, result.Status, result.Seconds, result.Bytes, result.SSIM, result.PSNR)
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
