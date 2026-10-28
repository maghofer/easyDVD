package media

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"easydvd"
	"easydvd/internal/process"
)

// The tested release family is intentionally bounded. A future major needs validation.
const SupportedFFmpeg = "FFmpeg 9.0.2 or later in the 9.x release family"

type FFmpegPair struct{ FFmpeg, FFprobe, Version string }
type version struct{ major, minor, patch int }

var releasePattern = regexp.MustCompile(`(?m)^(?:ffmpeg|ffprobe) version (\d+)\.(\d+)(?:\.(\d+))?(?:[-_\s]|$)`)

func parseVersion(s string) (version, error) {
	m := releasePattern.FindStringSubmatch(s)
	if len(m) == 0 {
		return version{}, fmt.Errorf("unrecognized FFmpeg release version")
	}
	a, _ := strconv.Atoi(m[1])
	b, _ := strconv.Atoi(m[2])
	c, _ := strconv.Atoi(m[3])
	return version{a, b, c}, nil
}
func (v version) String() string  { return fmt.Sprintf("%d.%d.%d", v.major, v.minor, v.patch) }
func (v version) supported() bool { return v.major == 9 && (v.minor > 0 || v.patch >= 2) }

type commandOutput func(context.Context, string, []string) (string, error)

func toolOutput(ctx context.Context, exe string, args []string) (string, error) {
	var b strings.Builder
	err := process.Run(ctx, exe, args, process.Options{Stdout: &b})
	return b.String(), err
}

func ValidateFFmpeg(ctx context.Context, pair FFmpegPair) (FFmpegPair, error) {
	return validateFFmpeg(ctx, pair, toolOutput)
}
func validateFFmpeg(ctx context.Context, pair FFmpegPair, run commandOutput) (FFmpegPair, error) {
	if pair.FFmpeg == "" || pair.FFprobe == "" {
		return pair, fmt.Errorf("both ffmpeg and ffprobe are required")
	}
	a, err := run(ctx, pair.FFmpeg, []string{"-version"})
	if err != nil {
		return pair, err
	}
	av, err := parseVersion(a)
	if err != nil {
		return pair, err
	}
	b, err := run(ctx, pair.FFprobe, []string{"-version"})
	if err != nil {
		return pair, err
	}
	bv, err := parseVersion(b)
	if err != nil {
		return pair, err
	}
	if av != bv {
		return pair, fmt.Errorf("FFmpeg %s and FFprobe %s do not match; choose a matching pair", av, bv)
	}
	if !av.supported() {
		return pair, fmt.Errorf("found FFmpeg %s; supported: %s", av, SupportedFFmpeg)
	}
	checks := []struct {
		arg   string
		names []string
	}{{"-encoders", []string{"mpeg2video", "ac3", "mjpeg"}}, {"-decoders", []string{"mpeg2video", "ac3", "png"}}, {"-filters", []string{"scale", "pad", "setsar", "fps", "bwdif", "apad", "anullsrc"}}, {"-muxers", []string{"dvd", "null", "image2"}}}
	for _, check := range checks {
		out, e := run(ctx, pair.FFmpeg, []string{"-hide_banner", check.arg})
		if e != nil {
			return pair, e
		}
		names := map[string]bool{}
		for _, line := range strings.Split(out, "\n") {
			f := strings.Fields(line)
			if len(f) > 1 {
				for _, name := range strings.Split(f[1], ",") {
					names[name] = true
				}
			}
		}
		for _, name := range check.names {
			if !names[name] {
				return pair, fmt.Errorf("FFmpeg %s lacks required %s capability %q", av, strings.TrimPrefix(check.arg, "-"), name)
			}
		}
	}
	pair.Version = av.String()
	return pair, nil
}

func FFmpegConfigPath() string {
	base, _ := os.UserConfigDir()
	return filepath.Join(base, "easyDVD", "ffmpeg.json")
}
func FFmpegCacheRoot() string {
	base, _ := os.UserCacheDir()
	return filepath.Join(base, "easyDVD", "ffmpeg")
}
func savedFFmpegFolder() string {
	b, err := os.ReadFile(FFmpegConfigPath())
	if err != nil {
		return ""
	}
	var cfg struct {
		Directory string `json:"directory"`
	}
	if json.Unmarshal(b, &cfg) != nil {
		return ""
	}
	return cfg.Directory
}
func SaveFFmpegFolder(dir string) error {
	if !filepath.IsAbs(dir) {
		return fmt.Errorf("FFmpeg folder must be an absolute path")
	}
	path := FFmpegConfigPath()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	b, _ := json.MarshalIndent(struct {
		Directory string `json:"directory"`
	}{dir}, "", "  ")
	f, err := os.CreateTemp(filepath.Dir(path), "ffmpeg-*.json.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if _, err = f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func pairAt(dir string) FFmpegPair {
	if absolute, err := filepath.Abs(dir); err == nil {
		dir = absolute
	}
	suffix := ""
	if os.PathSeparator == '\\' {
		suffix = ".exe"
	}
	return FFmpegPair{FFmpeg: filepath.Join(dir, "ffmpeg"+suffix), FFprobe: filepath.Join(dir, "ffprobe"+suffix)}
}
func pairExists(p FFmpegPair) bool {
	for _, path := range []string{p.FFmpeg, p.FFprobe} {
		st, err := os.Stat(path)
		if err != nil || st.IsDir() {
			return false
		}
	}
	return true
}
func ValidateFFmpegFolder(ctx context.Context, dir string) (FFmpegPair, error) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return FFmpegPair{}, err
	}
	for _, folder := range []string{dir, filepath.Join(dir, "bin")} {
		pair := pairAt(folder)
		if pairExists(pair) {
			return ValidateFFmpeg(ctx, pair)
		}
	}
	return FFmpegPair{}, fmt.Errorf("this folder does not contain ffmpeg and ffprobe (or a bin folder containing them)")
}

func ResolveTools(ctx context.Context, folder string) (Tools, error) {
	t := FindAuthoringTools()
	var candidates []FFmpegPair
	if folder != "" {
		p, err := ValidateFFmpegFolder(ctx, folder)
		if err != nil {
			t.FFmpegIssue = err.Error()
			return t, err
		}
		t.FFmpeg = p.FFmpeg
		t.FFprobe = p.FFprobe
		return t, nil
	}
	// Explicit choices override automatic discovery; each candidate is validated as a pair.
	for _, root := range []string{os.Getenv("EASYDVD_FFMPEG"), savedFFmpegFolder(), os.Getenv("EASYDVD_TOOLS")} {
		if root != "" {
			candidates = append(candidates, pairAt(root), pairAt(filepath.Join(root, "bin")))
		}
	}
	ff, ffErr := exec.LookPath("ffmpeg")
	if ffErr != nil {
		ff = ""
	}
	fp, fpErr := exec.LookPath("ffprobe")
	if fpErr != nil {
		fp = ""
	}
	if ff != "" {
		candidates = append(candidates, pairAt(filepath.Dir(ff)))
		if fp != "" {
			candidates = append(candidates, FFmpegPair{FFmpeg: ff, FFprobe: fp})
		}
	}
	exe, _ := os.Executable()
	cwd, _ := os.Getwd()
	recommended := easydvd.RecommendedFFmpeg()
	for _, root := range []string{filepath.Join(FFmpegCacheRoot(), recommended.Version, "bin"), filepath.Join(filepath.Dir(exe), "tools"), filepath.Join(cwd, ".tools", "media", "bin")} {
		candidates = append(candidates, pairAt(root))
	}
	return resolveCandidates(ctx, t, candidates, ValidateFFmpeg)
}
func resolveCandidates(ctx context.Context, t Tools, candidates []FFmpegPair, validate func(context.Context, FFmpegPair) (FFmpegPair, error)) (Tools, error) {
	seen := map[string]bool{}
	var failures []string
	for _, p := range candidates {
		if err := ctx.Err(); err != nil {
			return t, err
		}
		key := p.FFmpeg + "\x00" + p.FFprobe
		if seen[key] || !pairExists(p) {
			continue
		}
		seen[key] = true
		valid, err := validate(ctx, p)
		if err == nil {
			t.FFmpeg = valid.FFmpeg
			t.FFprobe = valid.FFprobe
			return t, nil
		}
		failures = append(failures, fmt.Sprintf("%s: %v", p.FFmpeg, err))
	}
	reason := "No compatible FFmpeg / FFprobe pair was found"
	if len(failures) > 0 {
		reason = strings.Join(failures, "\n")
	}
	t.FFmpegIssue = reason
	return t, fmt.Errorf("%s", reason)
}
