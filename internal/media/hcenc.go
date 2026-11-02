package media

import (
	"context"
	"debug/pe"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"easydvd/internal/model"
	"easydvd/internal/platform"
	"easydvd/internal/process"
)

// The portable Quality runtime is preferred to automatic external discovery.
// Explicit EASYDVD_HCENC / EASYDVD_TOOLS choices retain precedence.
func findHCenc() (encoder, plugin, avisynth string) {
	if runtime.GOOS != "windows" {
		return
	}
	exe, _ := os.Executable()
	cwd, _ := os.Getwd()
	roots := []string{os.Getenv("EASYDVD_HCENC"), os.Getenv("EASYDVD_TOOLS"), filepath.Join(filepath.Dir(exe), "tools", "quality"), filepath.Join(filepath.Dir(exe), "tools"), filepath.Join(cwd, ".tools", "media", "bin", "quality"), filepath.Join(cwd, ".tools", "media", "bin")}
	if p, err := exec.LookPath("HCenc.exe"); err == nil {
		roots = append(roots, filepath.Dir(p))
	}
	for _, env := range []string{"ProgramFiles(x86)", "ProgramFiles"} {
		if p := os.Getenv(env); p != "" {
			roots = append(roots, filepath.Join(p, "AVStoDVD"))
		}
	}
	for _, root := range roots {
		if root == "" {
			continue
		}
		explicit := root == os.Getenv("EASYDVD_HCENC")
		if st, err := os.Stat(filepath.Join(root, "quality")); err == nil && st.IsDir() {
			root = filepath.Join(root, "quality")
		}
		enc := firstFile(filepath.Join(root, "HCenc.exe"), filepath.Join(root, "HCenc", "HCenc.exe"))
		if enc == "" && !explicit && filepath.Base(root) == "quality" && directoryExists(root) {
			// The distributable support runtime uses a separately downloaded,
			// hash-verified HCEnc. Never borrow an unrelated system AviSynth.
			return ManagedHCencPath(), firstFile(filepath.Join(root, "ffms2.dll")), firstFile(filepath.Join(root, "avisynth.dll"))
		}
		if enc == "" {
			if explicit || filepath.Base(root) == "quality" && directoryExists(root) {
				return "", "", ""
			}
			continue
		}
		ffms := firstFile(filepath.Join(root, "ffms2.dll"), filepath.Join(root, "Lib", "ffms2.dll"), filepath.Join(root, "..", "Lib", "ffms2.dll"))
		avs := firstFile(filepath.Join(filepath.Dir(enc), "avisynth.dll"), filepath.Join(root, "avisynth.dll"))
		// A damaged portable runtime must not silently use a system installation.
		if avs == "" && filepath.Base(root) != "quality" {
			avs = firstFile(filepath.Join(os.Getenv("WINDIR"), "SysWOW64", "avisynth.dll"))
		}
		if encoder == "" {
			encoder, plugin, avisynth = enc, ffms, avs
		}
		// An explicit root must not silently select another installation.
		if explicit || filepath.Base(root) == "quality" || ffms != "" && avs != "" {
			return enc, ffms, avs
		}
	}
	return
}

func directoryExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && st.IsDir()
}

func firstFile(paths ...string) string {
	for _, p := range paths {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			absolute, err := filepath.Abs(p)
			if err == nil {
				return absolute
			}
		}
	}
	return ""
}

func (t Tools) RequireProfile(s model.Settings) error {
	if s.IsBluray() || s.Profile() != model.ProfileQuality {
		return nil
	}
	if runtime.GOOS != "windows" {
		return fmt.Errorf("Quality (hcenc) requires HCEnc 0.28 on Windows")
	}
	for _, component := range []struct{ name, path string }{{"HCenc.exe", t.HCenc}, {"FFMS2 (ffms2.dll)", t.FFMS2}, {"AviSynth", t.AviSynth}} {
		if component.path == "" {
			return fmt.Errorf("Quality (hcenc) runtime %s is missing. Open Quality (hcenc) setup to download HCEnc, or restore tools/quality for the included support libraries (see docs/ENCODERS.md)", component.name)
		}
		binary, err := pe.Open(component.path)
		if err != nil {
			return fmt.Errorf("Quality (hcenc) runtime %s: %w", component.name, err)
		}
		machine := binary.Machine
		binary.Close()
		if machine != pe.IMAGE_FILE_MACHINE_I386 {
			return fmt.Errorf("Quality (hcenc) requires 32-bit %s to match HCEnc 0.28", component.name)
		}
	}
	_, err := avisynthSupport(t.AviSynth)
	return err
}

// Classic AviSynth imports DevIL even when no image filters are used. Resolve
// it beside the selected DLL, never from an unrelated installed runtime.
func avisynthSupport(path string) ([]string, error) {
	file, err := pe.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	libs, err := peImportLibraries(file)
	if err != nil {
		return nil, err
	}
	for _, lib := range libs {
		if strings.EqualFold(lib, "DevIL.dll") {
			support := firstFile(filepath.Join(filepath.Dir(path), "DevIL.dll"))
			if support == "" {
				return nil, fmt.Errorf("Quality (hcenc) runtime DevIL.dll is missing beside avisynth.dll; re-extract the complete easyDVD package")
			}
			dep, err := pe.Open(support)
			if err != nil {
				return nil, fmt.Errorf("Quality (hcenc) runtime DevIL.dll: %w", err)
			}
			machine := dep.Machine
			dep.Close()
			if machine != pe.IMAGE_FILE_MACHINE_I386 {
				return nil, fmt.Errorf("Quality (hcenc) requires 32-bit DevIL.dll to match AviSynth")
			}
			return []string{support}, nil
		}
	}
	return nil, nil
}

// debug/pe.ImportedLibraries is unimplemented, and ImportedSymbols skips
// descriptors with no OriginalFirstThunk (used by the packed AviSynth DLL).
// Read dependency names directly; no function/ordinal resolution is needed.
func peImportLibraries(file *pe.File) ([]string, error) {
	var table pe.DataDirectory
	switch header := file.OptionalHeader.(type) {
	case *pe.OptionalHeader32:
		table = header.DataDirectory[pe.IMAGE_DIRECTORY_ENTRY_IMPORT]
	case *pe.OptionalHeader64:
		table = header.DataDirectory[pe.IMAGE_DIRECTORY_ENTRY_IMPORT]
	default:
		return nil, fmt.Errorf("runtime DLL has no PE optional header")
	}
	if table.VirtualAddress == 0 {
		return nil, nil
	}
	dataAt := func(rva uint32) ([]byte, error) {
		for _, section := range file.Sections {
			if section.Offset == 0 || rva < section.VirtualAddress || rva-section.VirtualAddress >= section.Size {
				continue
			}
			data, err := section.Data()
			if err != nil {
				return nil, err
			}
			offset := rva - section.VirtualAddress
			if uint64(offset) < uint64(len(data)) {
				return data[offset:], nil
			}
		}
		return nil, fmt.Errorf("runtime DLL import address is outside its file sections")
	}
	descriptors, err := dataAt(table.VirtualAddress)
	if err != nil {
		return nil, err
	}
	var libraries []string
	for i := 0; i < 512 && len(descriptors) >= 20; i++ {
		nameRVA := binary.LittleEndian.Uint32(descriptors[12:16])
		if nameRVA == 0 && binary.LittleEndian.Uint32(descriptors[:4]) == 0 && binary.LittleEndian.Uint32(descriptors[16:20]) == 0 {
			return libraries, nil
		}
		name, err := dataAt(nameRVA)
		if err != nil {
			return nil, err
		}
		end := strings.IndexByte(string(name[:min(len(name), 256)]), 0)
		if end <= 0 {
			return nil, fmt.Errorf("runtime DLL has an invalid import name")
		}
		libraries = append(libraries, string(name[:end]))
		descriptors = descriptors[20:]
	}
	return nil, fmt.Errorf("runtime DLL has an unterminated import table")
}

// This is the HCEnc configuration from the supplied AVStoDVD 2.8.9 log.
// Frame type/field order follow the actual prepared video, rather than forcing
// the progressive PAL example onto interlaced or NTSC sources. -2pass on the
// command line explicitly prevents single-pass/CQ encoding.
func HCencINI(v model.Video, s model.Settings, interlaced bool) string {
	frame, order := "*PROGRESSIVE", "*TFF"
	if interlaced {
		frame = "*INTERLACED"
		if v.FieldOrder == "bb" || v.FieldOrder == "bt" {
			order = "*BFF"
		}
	}
	return fmt.Sprintf("*INFILE input.avs\n*AVSRELOAD\n*OUTFILE video.m2v\n*LOGFILE hcenc.log\n*MAXBITRATE 9000\n*BITRATE %d\n*PROFILE BEST\n*ASPECT %s\n%s\n%s\n*BIAS 80\n*DC_PREC 10\n*MATRIX MPEG\n*AQ 1\n*LUMGAIN 1\n*SMP\n*PRIORITY IDLE\n*DBPATH .\n*WAIT 0\n", s.VideoBitrate()/1000, v.Aspect(), frame, order)
}

func HCencPrepareArgs(v model.Video, s model.Settings, input, output string) []string {
	filter, _ := VideoFilter(v, s)
	return append(ffmpegInputArgs(s), "-i", input,
		"-map", fmt.Sprintf("0:%d", v.VideoStream), "-vf", filter, "-an", "-sn", "-dn", "-t", fmt.Sprintf("%.6f", v.Duration),
		"-c:v", "ffv1", "-level", "1", "-threads:v", fmt.Sprint(encodingThreads(s)), "-pix_fmt", "yuv420p", "-progress", "pipe:1", "-nostats", output)
}

func HCencMuxArgs(v model.Video, s model.Settings, input, video, output string) []string {
	args := []string{"-hide_banner", "-nostdin", "-y", "-xerror", "-fflags", "+genpts", "-r", s.FPS(), "-i", video}
	audio := "1:a:0"
	if v.AudioStream < 0 {
		args = append(args, "-f", "lavfi", "-i", "anullsrc=r=48000:cl=stereo")
	} else {
		args = append(args, "-err_detect", "explode", "-i", input)
		audio = fmt.Sprintf("1:%d", v.AudioStream)
	}
	// The prepared video already has the intended duration. An output -t can
	// stop stream copying before delayed/reordered MPEG-2 pictures are written.
	// Drain it to EOF and let padded audio follow it via -shortest instead.
	// Shift both timestamps by the first PTS, preserving B-picture decode order
	// while aligning the first displayed video frame with the source audio.
	return append(args, "-map", "0:v:0", "-map", audio, "-c:v", "copy", "-bsf:v", "setts=pts=PTS-STARTPTS:dts=DTS-STARTPTS",
		"-c:a", "ac3", "-ar", "48000", "-ac", "2", "-b:a", "448k", "-af", "apad", "-shortest",
		"-sn", "-dn", "-progress", "pipe:1", "-nostats", "-f", "dvd", "-muxrate", "10080k", "-packetsize", "2048", output)
}

func hcencFrameCount(report string) (int, error) {
	for _, line := range strings.Split(report, "\n") {
		name, value, ok := strings.Cut(line, ":")
		if !ok || strings.TrimSpace(name) != "nr. of frames" {
			continue
		}
		frames, err := strconv.Atoi(strings.TrimSpace(value))
		if err == nil && frames > 0 {
			return frames, nil
		}
		break
	}
	return 0, fmt.Errorf("HCEnc did not report a valid encoded frame count")
}

func (t Tools) encodeHCenc(ctx context.Context, v model.Video, s model.Settings, input, dir, output string, log io.Writer, progress func(float64)) error {
	// Each attempt gets a fresh directory. A zero exit code from a failed legacy
	// encoder must never allow a previous attempt's elementary stream to survive.
	work, err := os.MkdirTemp(dir, "hcenc-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)
	prepared := filepath.Join(work, "input.mkv")
	if log != nil {
		fmt.Fprintln(log, "Quality (hcenc): HCEnc 0.28, AVStoDVD BEST / two-pass VBR / 8500 target / 9000 peak")
	}
	if err := process.Run(ctx, t.FFmpeg, HCencPrepareArgs(v, s, input, prepared), process.Options{Log: log, Line: func(line string) {
		if progress != nil && strings.HasPrefix(line, "out_time_us=") {
			var us float64
			fmt.Sscanf(line, "out_time_us=%f", &us)
			progress(math.Min(.15, .15*us/1e6/v.Duration))
		}
	}}); err != nil {
		return fmt.Errorf("prepare HCEnc input: %w", err)
	}
	// Legacy tools need ASCII paths independently of the Go app's UTF-8 manifest.
	legacy, cleanup, err := platform.LegacyWorkingDir(work)
	if err != nil {
		return err
	}
	defer cleanup()
	pluginDir, cleanupPlugin, err := platform.LegacyWorkingDir(filepath.Dir(t.FFMS2))
	if err != nil {
		return err
	}
	defer cleanupPlugin()
	plugin := filepath.Join(pluginDir, filepath.Base(t.FFMS2))
	if strings.ContainsAny(plugin, "\"\r\n") {
		return fmt.Errorf("invalid FFMS2 plugin path")
	}
	// FFMS2 exists as both a C++ plugin and (as supplied by AVStoDVD) a C plugin.
	avs := fmt.Sprintf("try { LoadPlugin(\"%s\") } catch (e) { LoadCPlugin(\"%s\") }\nFFVideoSource(\"input.mkv\", track=0, cachefile=\"input.ffindex\", threads=1, colorspace=\"YV12\")\n", plugin, plugin)
	_, interlaced := VideoFilter(v, s)
	if interlaced {
		if v.FieldOrder == "bb" || v.FieldOrder == "bt" {
			avs += "AssumeBFF()\n"
		} else {
			avs += "AssumeTFF()\n"
		}
	}
	ini := HCencINI(v, s, interlaced)
	for name, contents := range map[string]string{"input.avs": avs, "hcenc.ini": ini} {
		if err := os.WriteFile(filepath.Join(work, name), []byte(contents), 0600); err != nil {
			return err
		}
	}
	if log != nil {
		fmt.Fprintln(log, "HCEnc input script:\n"+avs+"HCEnc INI:\n"+ini)
	}
	// HCEnc's -noini also disables -ini in the tested 0.28.0.2 build.
	// Run a private executable copy so a user's default HC.ini beside the
	// installed encoder cannot leak settings into this encode.
	runtimeFiles := map[string]string{"HCenc.exe": t.HCenc, "avisynth.dll": t.AviSynth}
	support, err := avisynthSupport(t.AviSynth)
	if err != nil {
		return err
	}
	for _, source := range support {
		runtimeFiles[filepath.Base(source)] = source
	}
	for name, source := range runtimeFiles {
		binary, err := os.ReadFile(source)
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(work, name), binary, 0700); err != nil {
			return err
		}
	}
	err = process.Run(ctx, filepath.Join(legacy, "HCenc.exe"), []string{"-ini", filepath.Join(legacy, "hcenc.ini"), "-2pass", "-silent", "-wait", "0"}, process.Options{Dir: legacy, Log: log, Stdout: log, Env: []string{"TEMP=" + legacy, "TMP=" + legacy, "PATH=" + filepath.Dir(t.AviSynth) + string(os.PathListSeparator) + os.Getenv("PATH")}})
	hcLog, logErr := os.ReadFile(filepath.Join(work, "hcenc.log"))
	if log != nil {
		fmt.Fprintf(log, "\nHCEnc report:\n%s\n", hcLog)
	}
	if err != nil {
		return fmt.Errorf("HCEnc encoding (check that HCEnc, AviSynth and FFMS2 can open the generated input script): %w\n%s", err, hcLog)
	}
	report := strings.ToLower(string(hcLog))
	if logErr != nil || !strings.Contains(report, "pass 2 encoding time:") || !strings.Contains(report, "bytes in bitstream:") {
		return fmt.Errorf("HCEnc did not complete successfully: %v\n%s", logErr, hcLog)
	}
	frames, err := hcencFrameCount(report)
	if err != nil {
		return fmt.Errorf("%w\n%s", err, hcLog)
	}
	video := filepath.Join(work, "video.m2v")
	if st, err := os.Stat(video); err != nil || st.Size() == 0 {
		return fmt.Errorf("HCEnc did not produce video: %v\n%s", err, hcLog)
	}
	if progress != nil {
		progress(.9)
	}
	muxed := 0
	if err := process.Run(ctx, t.FFmpeg, HCencMuxArgs(v, s, input, video, output), process.Options{Log: log, Line: func(line string) {
		if strings.HasPrefix(line, "frame=") {
			n, _ := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, "frame=")))
			muxed = max(muxed, n)
		}
	}}); err != nil {
		return fmt.Errorf("mux HCEnc video and AC-3 audio: %w", err)
	}
	if log != nil {
		fmt.Fprintf(log, "HCEnc frame preservation: %d encoded / %d muxed\n", frames, muxed)
	}
	if muxed != frames {
		return fmt.Errorf("HCEnc mux frame count mismatch: encoded %d, muxed %d", frames, muxed)
	}
	if progress != nil {
		progress(1)
	}
	return nil
}

// Reserve a raw-frame upper bound plus FFV1 container overhead and the
// elementary MPEG-2 stream. The lossless input is removed after each title.
func HCencScratchBytes(v model.Video, s model.Settings) int64 {
	return int64(math.Ceil(v.Duration*Ratio(s.FPS())))*720*int64(s.Height())*3/2*105/100 + v.EstimatedBytes(s)
}
