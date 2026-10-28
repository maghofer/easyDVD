package media

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"easydvd"
)

func digest(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func fixtureInstaller(t *testing.T) (installOptions, *atomic.Int32) {
	t.Helper()
	archive := []byte("pinned archive fixture")
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(archive) }))
	t.Cleanup(server.Close)
	ff, probe := []byte("ffmpeg fixture"), []byte("ffprobe fixture")
	extractions := &atomic.Int32{}
	opt := installOptions{root: t.TempDir(), release: easydvd.FFmpegRelease{Version: "9.0.2", URL: server.URL, SHA256: digest(archive), FFmpegExeSHA256: digest(ff), FFprobeExeSHA256: digest(probe)}, client: server.Client(), extract: func(ctx context.Context, archive, dir string) error {
		extractions.Add(1)
		pair := pairAt(filepath.Join(dir, "ffmpeg-9.0.2", "bin"))
		if err := os.MkdirAll(filepath.Dir(pair.FFmpeg), 0700); err != nil {
			return err
		}
		if err := os.WriteFile(pair.FFmpeg, ff, 0600); err != nil {
			return err
		}
		return os.WriteFile(pair.FFprobe, probe, 0600)
	}, validate: func(ctx context.Context, p FFmpegPair) (FFmpegPair, error) {
		if !pairExists(p) {
			return p, fmt.Errorf("missing pair")
		}
		p.Version = "9.0.2"
		return p, nil
	}}
	return opt, extractions
}
func TestInstallPublishesOnlyValidatedPair(t *testing.T) {
	opt, count := fixtureInstaller(t)
	p, err := installFFmpeg(context.Background(), opt, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !pinnedPair(context.Background(), p, opt.release) || p.Version != "9.0.2" || count.Load() != 1 {
		t.Fatal(p, count.Load())
	}
	again, err := installFFmpeg(context.Background(), opt, nil)
	if err != nil || again.FFmpeg != p.FFmpeg || count.Load() != 1 {
		t.Fatal("valid installation was not reused", again, err)
	}
	entries, _ := os.ReadDir(opt.root)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".install-") {
			t.Fatal("staging directory was not cleaned")
		}
	}
}
func TestChecksumFailureNeverExtracts(t *testing.T) {
	opt, count := fixtureInstaller(t)
	opt.release.SHA256 = strings.Repeat("0", 64)
	_, err := installFFmpeg(context.Background(), opt, nil)
	if err == nil || !strings.Contains(err.Error(), "checksum") || count.Load() != 0 {
		t.Fatal(err, count.Load())
	}
	if _, err = os.Stat(filepath.Join(opt.root, "9.0.2")); !os.IsNotExist(err) {
		t.Fatal("failed download was published")
	}
}
func TestFailedCompatibilityDoesNotPublish(t *testing.T) {
	opt, _ := fixtureInstaller(t)
	opt.validate = func(ctx context.Context, p FFmpegPair) (FFmpegPair, error) { return p, fmt.Errorf("missing encoder") }
	_, err := installFFmpeg(context.Background(), opt, nil)
	if err == nil {
		t.Fatal("incompatible installation accepted")
	}
	if _, err = os.Stat(filepath.Join(opt.root, "9.0.2")); !os.IsNotExist(err) {
		t.Fatal("incompatible installation was published")
	}
}
func TestExistingVersionIsNeverOverwritten(t *testing.T) {
	opt, _ := fixtureInstaller(t)
	old := pairAt(filepath.Join(opt.root, "9.0.2", "bin"))
	os.MkdirAll(filepath.Dir(old.FFmpeg), 0700)
	os.WriteFile(old.FFmpeg, []byte("old tool"), 0600)
	os.WriteFile(old.FFprobe, []byte("old probe"), 0600)
	p, err := installFFmpeg(context.Background(), opt, nil)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(old.FFmpeg)
	if string(b) != "old tool" || p.FFmpeg == old.FFmpeg {
		t.Fatal("existing version overwritten")
	}
}
func TestDownloadCancellationCleansPartialFiles(t *testing.T) {
	opt, count := fixtureInstaller(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "100000")
		w.Write([]byte("partial"))
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	defer server.Close()
	opt.release.URL = server.URL
	opt.client = server.Client()
	_, err := installFFmpeg(ctx, opt, func(p DownloadProgress) {
		if p.Downloaded > 0 {
			cancel()
		}
	})
	if err == nil || count.Load() != 0 {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(opt.root)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".install-") {
			t.Fatal("canceled download left partial files")
		}
	}
}
func TestPinnedVersionCannotEscapeCache(t *testing.T) {
	opt, _ := fixtureInstaller(t)
	for _, v := range []string{"..", "../other", "1..2", ""} {
		opt.release.Version = v
		if _, err := installFFmpeg(context.Background(), opt, nil); err == nil {
			t.Fatal("invalid version accepted", v)
		}
	}
}
