package media

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"easydvd"
	"easydvd/internal/platform"
	"easydvd/internal/process"
)

type DownloadProgress struct {
	Stage             string
	Downloaded, Total int64
}
type installOptions struct {
	root     string
	release  easydvd.FFmpegRelease
	client   *http.Client
	extract  func(context.Context, string, string) error
	validate func(context.Context, FFmpegPair) (FFmpegPair, error)
}

// InstallRecommendedFFmpeg is called only by an explicit Download action or CLI flag.
func InstallRecommendedFFmpeg(ctx context.Context, progress func(DownloadProgress)) (FFmpegPair, error) {
	if runtime.GOOS != "windows" {
		return FFmpegPair{}, fmt.Errorf("automatic FFmpeg setup currently supports Windows; select an existing installation")
	}
	transport := &http.Transport{Proxy: http.ProxyFromEnvironment, DialContext: (&net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second}).DialContext, TLSHandshakeTimeout: 15 * time.Second, ResponseHeaderTimeout: 30 * time.Second, IdleConnTimeout: 60 * time.Second}
	client := &http.Client{Transport: transport, Timeout: 30 * time.Minute, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if req.URL.Scheme != "https" {
			return fmt.Errorf("FFmpeg download must use HTTPS")
		}
		if len(via) > 10 {
			return fmt.Errorf("too many download redirects")
		}
		return nil
	}}
	defer transport.CloseIdleConnections()
	extract := func(ctx context.Context, archive, dest string) error {
		tar := filepath.Join(os.Getenv("SystemRoot"), "System32", "tar.exe")
		if _, err := os.Stat(tar); err != nil {
			tar, err = exec.LookPath("tar.exe")
			if err != nil {
				return fmt.Errorf("Windows tar.exe is unavailable; download and extract FFmpeg manually, then use Locate")
			}
		}
		return process.Run(ctx, tar, []string{"-xf", archive, "-C", dest}, process.Options{})
	}
	return installFFmpeg(ctx, installOptions{FFmpegCacheRoot(), easydvd.RecommendedFFmpeg(), client, extract, ValidateFFmpeg}, progress)
}

func installFFmpeg(ctx context.Context, opt installOptions, progress func(DownloadProgress)) (FFmpegPair, error) {
	if err := ctx.Err(); err != nil {
		return FFmpegPair{}, err
	}
	if !regexpVersion(opt.release.Version) {
		return FFmpegPair{}, fmt.Errorf("invalid pinned FFmpeg version")
	}
	parsed, err := url.Parse(opt.release.URL)
	if err != nil || parsed.Scheme != "https" {
		return FFmpegPair{}, fmt.Errorf("FFmpeg download must use HTTPS")
	}
	if err = os.MkdirAll(opt.root, 0700); err != nil {
		return FFmpegPair{}, err
	}
	unlock, err := platform.LockFile(filepath.Join(opt.root, "install.lock"))
	if err != nil {
		return FFmpegPair{}, fmt.Errorf("another FFmpeg setup is already running: %w", err)
	}
	defer unlock()
	canonical := filepath.Join(opt.root, opt.release.Version)
	existing := pairAt(filepath.Join(canonical, "bin"))
	if pinnedPair(ctx, existing, opt.release) {
		if p, e := opt.validate(ctx, existing); e == nil {
			return p, nil
		}
	}
	stage, err := os.MkdirTemp(opt.root, ".install-")
	if err != nil {
		return FFmpegPair{}, err
	}
	defer os.RemoveAll(stage)
	emit := func(s string, done, total int64) {
		if progress != nil {
			progress(DownloadProgress{s, done, total})
		}
	}
	archive := filepath.Join(stage, "download.7z")
	if err = downloadArchive(ctx, opt.client, opt.release.URL, opt.release.SHA256, archive, progress); err != nil {
		return FFmpegPair{}, err
	}
	extracted := filepath.Join(stage, "extracted")
	if err = os.Mkdir(extracted, 0700); err != nil {
		return FFmpegPair{}, err
	}
	emit("Extracting FFmpeg", 0, 0)
	if err = opt.extract(ctx, archive, extracted); err != nil {
		return FFmpegPair{}, err
	}
	var source string
	err = filepath.WalkDir(extracted, func(path string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if d.IsDir() && pairExists(pairAt(path)) {
			if source != "" {
				return fmt.Errorf("archive contains more than one FFmpeg pair")
			}
			source = path
			return filepath.SkipDir
		}
		return nil
	})
	if err != nil {
		return FFmpegPair{}, err
	}
	if source == "" {
		return FFmpegPair{}, fmt.Errorf("downloaded archive did not contain FFmpeg and FFprobe")
	}
	sourcePair := pairAt(source)
	if !pinnedPair(ctx, sourcePair, opt.release) {
		return FFmpegPair{}, fmt.Errorf("extracted FFmpeg binary checksum mismatch")
	}
	ready := filepath.Join(stage, "ready")
	if err = os.MkdirAll(filepath.Join(ready, "bin"), 0700); err != nil {
		return FFmpegPair{}, err
	}
	emit("Installing FFmpeg", 0, 0)
	targetPair := pairAt(filepath.Join(ready, "bin"))
	for _, files := range [][2]string{{sourcePair.FFmpeg, targetPair.FFmpeg}, {sourcePair.FFprobe, targetPair.FFprobe}} {
		if err = copyDownloaded(ctx, files[0], files[1]); err != nil {
			return FFmpegPair{}, err
		}
	}
	for _, name := range []string{"LICENSE", "README.txt"} {
		if _, e := os.Stat(filepath.Join(filepath.Dir(source), name)); e == nil {
			if err = copyDownloaded(ctx, filepath.Join(filepath.Dir(source), name), filepath.Join(ready, name)); err != nil {
				return FFmpegPair{}, err
			}
		}
	}
	emit("Checking FFmpeg compatibility", 0, 0)
	pair, err := opt.validate(ctx, targetPair)
	if err != nil {
		return FFmpegPair{}, err
	}
	if err = ctx.Err(); err != nil {
		return FFmpegPair{}, err
	}
	// Never overwrite a tool version another running job might still be using.
	final := canonical
	if _, e := os.Stat(final); !os.IsNotExist(e) {
		final = filepath.Join(opt.root, opt.release.Version+"-"+strings.TrimPrefix(filepath.Base(stage), ".install-"))
	}
	if err = os.Rename(ready, final); err != nil {
		return FFmpegPair{}, err
	}
	pair.FFmpeg = pairAt(filepath.Join(final, "bin")).FFmpeg
	pair.FFprobe = pairAt(filepath.Join(final, "bin")).FFprobe
	emit("FFmpeg is ready", 0, 0)
	return pair, nil
}

func regexpVersion(v string) bool {
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return false
	}
	for _, p := range parts {
		if p == "" {
			return false
		}
		for _, c := range p {
			if c < '0' || c > '9' {
				return false
			}
		}
	}
	return true
}
func pinnedPair(ctx context.Context, p FFmpegPair, r easydvd.FFmpegRelease) bool {
	if !pairExists(p) {
		return false
	}
	a, err := HashFile(ctx, p.FFmpeg)
	if err != nil || a != r.FFmpegExeSHA256 {
		return false
	}
	b, err := HashFile(ctx, p.FFprobe)
	return err == nil && b == r.FFprobeExeSHA256
}
func copyDownloaded(ctx context.Context, src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	buf := make([]byte, 1024*1024)
	for {
		if err = ctx.Err(); err != nil {
			break
		}
		n, e := in.Read(buf)
		if n > 0 {
			if _, err = out.Write(buf[:n]); err != nil {
				break
			}
		}
		if e == io.EOF {
			break
		}
		if e != nil {
			err = e
			break
		}
	}
	if err == nil {
		err = out.Sync()
	}
	closeErr := out.Close()
	if err != nil {
		return err
	}
	return closeErr
}

func downloadArchive(ctx context.Context, client *http.Client, source, expected, path string, progress func(DownloadProgress)) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, source, nil)
	if err != nil {
		return err
	}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("FFmpeg download returned HTTP %d", response.StatusCode)
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	hash := sha256.New()
	writer := io.MultiWriter(file, hash)
	buf := make([]byte, 256*1024)
	var downloaded int64
	last := time.Time{}
	for {
		if err = ctx.Err(); err != nil {
			return err
		}
		n, e := response.Body.Read(buf)
		if n > 0 {
			if _, err = writer.Write(buf[:n]); err != nil {
				return err
			}
			downloaded += int64(n)
			if progress != nil && time.Since(last) >= 100*time.Millisecond {
				progress(DownloadProgress{"Downloading FFmpeg", downloaded, response.ContentLength})
				last = time.Now()
			}
		}
		if e == io.EOF {
			break
		}
		if e != nil {
			return e
		}
	}
	if response.ContentLength >= 0 && downloaded != response.ContentLength {
		return fmt.Errorf("FFmpeg download was incomplete")
	}
	actual := hex.EncodeToString(hash.Sum(nil))
	if actual != expected {
		return fmt.Errorf("FFmpeg archive checksum mismatch; nothing was installed")
	}
	return file.Sync()
}
