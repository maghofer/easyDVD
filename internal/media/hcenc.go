package media

import (
	"context"
	"debug/pe"
	"easydvd/internal/model"
	"easydvd/internal/platform"
	"easydvd/internal/process"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

func (t Tools) HCencFolder(s model.Settings) string {
	if s.HCencDir != "" {
		return s.HCencDir
	}
	if folder := os.Getenv("EASYDVD_HCENC"); folder != "" {
		if absolute, err := filepath.Abs(folder); err == nil {
			return absolute
		}
		return folder
	}
	exe, _ := os.Executable()
	cwd, _ := os.Getwd()
	for _, folder := range []string{filepath.Join(filepath.Dir(exe), "tools", "hcenc"), filepath.Join(cwd, ".tools", "hcenc", "bin")} {
		if _, err := os.Stat(filepath.Join(folder, "HCenc_028.exe")); err == nil {
			return folder
		}
	}
	return ""
}

func (t Tools) RequireEncoder(s model.Settings) error {
	if s.Encoder() != "HCenc" {
		return nil
	}
	folder := t.HCencFolder(s)
	if folder == "" {
		return fmt.Errorf("HCenc setup required: choose the HCenc folder in Settings (HCenc_028.exe, 32-bit AviSynth.dll and ffms2.dll), set EASYDVD_HCENC, or select FFmpeg as video encoder")
	}
	for _, name := range []string{"HCenc_028.exe", "AviSynth.dll", "ffms2.dll"} {
		file, err := pe.Open(filepath.Join(folder, name))
		if err != nil {
			return fmt.Errorf("HCenc setup: %s: %w", name, err)
		}
		machine := file.Machine
		file.Close()
		if machine != pe.IMAGE_FILE_MACHINE_I386 {
			return fmt.Errorf("HCenc setup: %s must be 32-bit (x86)", name)
		}
	}
	return nil
}

func (t Tools) EncoderStatus(s model.Settings) ToolStatus {
	status := ToolStatus{Name: "Video encoder", Version: s.Encoder() + " · two-pass MPEG-2"}
	if s.Encoder() == "HCenc" {
		status.Path = t.HCencFolder(s)
		if err := t.RequireEncoder(s); err != nil {
			status.Error = err.Error()
		}
	} else {
		status.Path = t.FFmpeg
	}
	return status
}

// HCenc performs both VBR passes in one invocation. No one-pass/CQ directives
// may be added here. FFmpeg only prepares lossless frames and encodes audio.
func HCencINI(v model.Video, s model.Settings) string {
	_, interlaced := VideoFilter(v, s)
	mode := "*PROGRESSIVE\n*PROG_SEQ\n"
	if interlaced {
		mode = "*INTERLACED\n*TFF\n"
		if v.FieldOrder == "bb" || v.FieldOrder == "bt" {
			mode = "*INTERLACED\n*BFF\n"
		}
	}
	colour := 5
	if s.Standard == "NTSC" {
		colour = 6
	}
	return fmt.Sprintf("*INFILE source.avs\n*OUTFILE video.m2v\n*LOGFILE hcenc.log\n*BITRATE 8000\n*MAXBITRATE 9000\n*PROFILE BEST\n*ASPECT %s\n*AUTOGOP %s\n*CLOSEDGOPS\n*DC_PREC 10\n*CHROMAFORMAT 420\n*COLOUR %d\n%s*SILENT\n*WAIT 0\n", v.Aspect(), s.GOP(), colour, mode)
}

func (t Tools) encodeHCenc(ctx context.Context, v model.Video, s model.Settings, input, dir string, log io.Writer, progress func(float64)) error {
	if err := t.RequireEncoder(s); err != nil {
		return err
	}
	// Use ASCII relative names in the legacy encoder's script and INI. Go and
	// FFmpeg still handle Unicode source/work paths through Windows' Unicode APIs.
	frames := filepath.Join(dir, "frames.mkv")
	plugin := filepath.Join(dir, "ffms2.dll")
	for _, name := range []string{"video.m2v", "hcenc.log"} {
		if err := os.Remove(filepath.Join(dir, name)); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	defer os.Remove(frames)
	defer os.Remove(plugin)
	defer os.Remove(filepath.Join(dir, "video.m2v"))
	sourcePlugin, err := os.Open(filepath.Join(t.HCencFolder(s), "ffms2.dll"))
	if err != nil {
		return err
	}
	destPlugin, err := os.Create(plugin)
	if err != nil {
		sourcePlugin.Close()
		return err
	}
	_, copyErr := io.Copy(destPlugin, sourcePlugin)
	sourcePlugin.Close()
	closeErr := destPlugin.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	filter, interlaced := VideoFilter(v, s)
	preparedFrames := 0
	args := []string{"-hide_banner", "-nostdin", "-y", "-xerror", "-err_detect", "explode", "-i", input, "-map", fmt.Sprintf("0:%d", v.VideoStream), "-vf", filter, "-an", "-sn", "-dn", "-c:v", "ffv1", "-level", "3", "-pix_fmt", "yuv420p", "-progress", "pipe:1", "-nostats", frames}
	if err = process.Run(ctx, t.FFmpeg, args, process.Options{Log: log, Line: func(line string) {
		if strings.HasPrefix(line, "frame=") {
			preparedFrames, _ = strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, "frame=")))
		}
		if progress != nil && strings.HasPrefix(line, "out_time_us=") {
			var us float64
			fmt.Sscanf(line, "out_time_us=%f", &us)
			progress(math.Min(.25, .25*us/1e6/v.Duration))
		}
	}}); err != nil {
		return err
	}
	script := "LoadPlugin(\"ffms2.dll\")\nFFVideoSource(\"frames.mkv\", cache=false)\n"
	if interlaced {
		if v.FieldOrder == "bb" || v.FieldOrder == "bt" {
			script += "AssumeBFF()\n"
		} else {
			script += "AssumeTFF()\n"
		}
	} else {
		script += "AssumeFrameBased()\n"
	}
	if err = os.WriteFile(filepath.Join(dir, "source.avs"), []byte(script), 0644); err != nil {
		return err
	}
	if err = os.WriteFile(filepath.Join(dir, "HC.ini"), []byte(HCencINI(v, s)), 0644); err != nil {
		return err
	}
	if progress != nil {
		progress(.25)
	}
	toolDir, cleanup, err := platform.LegacyWorkingDir(dir)
	if err != nil {
		return err
	}
	defer cleanup()
	err = process.Run(ctx, filepath.Join(t.HCencFolder(s), "HCenc_028.exe"), []string{"-ini", "HC.ini"}, process.Options{Dir: toolDir, Log: log, Stdout: log})
	// HCenc can return success after an input/configuration error. Require its
	// elementary stream as well as successful decode/metadata validation below.
	hcLog, _ := os.ReadFile(filepath.Join(dir, "hcenc.log"))
	if log != nil {
		_, _ = log.Write(hcLog)
	}
	if err != nil {
		return fmt.Errorf("HCenc: %w", err)
	}
	if err = validateHCencLog(string(hcLog), preparedFrames); err != nil {
		return err
	}
	video := filepath.Join(dir, "video.m2v")
	if st, e := os.Stat(video); e != nil || st.Size() == 0 {
		return fmt.Errorf("HCenc produced no video; check the 32-bit AviSynth runtime and FFMS2 plugin: %s", hcLog)
	}
	if progress != nil {
		progress(.85)
	}
	args = []string{"-hide_banner", "-nostdin", "-y", "-xerror", "-i", video, "-i", input}
	audio := fmt.Sprintf("1:%d", v.AudioStream)
	if v.AudioStream < 0 {
		args = append(args, "-f", "lavfi", "-i", "anullsrc=r=48000:cl=stereo")
		audio = "2:a:0"
	}
	args = append(args, "-map", "0:v:0", "-map", audio, "-c:v", "copy", "-c:a", "ac3", "-ar", "48000", "-ac", "2", "-b:a", "256k", "-af", "apad", "-shortest", "-t", fmt.Sprintf("%.6f", v.Duration), "-f", "dvd", "-muxrate", "10080k", "-packetsize", "2048", filepath.Join(dir, "video.part.mpg"))
	if err = process.Run(ctx, t.FFmpeg, args, process.Options{Log: log}); err != nil {
		return err
	}
	if progress != nil {
		progress(1)
	}
	return nil
}

var hcFrameCount = regexp.MustCompile(`(?m)^nr\. of frames:\s+(\d+)\s*$`)

func validateHCencLog(log string, preparedFrames int) error {
	frames := hcFrameCount.FindStringSubmatch(log)
	count := 0
	if len(frames) > 1 {
		count, _ = strconv.Atoi(frames[1])
	}
	if preparedFrames <= 0 || count != preparedFrames || !strings.Contains(log, "encoding - pass 1") || !strings.Contains(log, "encoding - pass 2") {
		return fmt.Errorf("HCenc did not complete both passes for all %d prepared frames; inspect the job log", preparedFrames)
	}
	return nil
}
