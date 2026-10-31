package media

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"easydvd/internal/platform"
)

const hcencVersion = "0.28.0.2"
const hcencArchiveURL = "http://hank315.nl/files/HC_028/HC028_21-12-2015.zip"
const hcencArchiveSHA256 = "ed392df990120cd5430454bec68555dd444f1193f930e021c42b1871f5e245a7"
const hcencExeSHA256 = "be55e5811c3641b364ff1e12e758b76895e61008e5759c2c14acc3ec4ca4f371"

func HCencCacheRoot() string {
	base, _ := os.UserCacheDir()
	return filepath.Join(base, "easyDVD", "hcenc")
}

// ManagedHCencPath returns only a byte-verified encoder from the private cache.
func ManagedHCencPath() string {
	return managedHCencAt(HCencCacheRoot())
}

func managedHCencAt(root string) string {
	return managedHCencWithHash(root, hcencExeSHA256)
}

func managedHCencWithHash(root, expected string) string {
	entries, _ := os.ReadDir(root)
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), hcencVersion) {
			continue
		}
		path := filepath.Join(root, entry.Name(), "HCenc.exe")
		if hash, err := HashFile(context.Background(), path); err == nil && hash == expected {
			return path
		}
	}
	return ""
}

// InstallRecommendedHCenc downloads the publisher's original ZIP. The legacy
// server uses HTTP; pinned archive and executable hashes are checked before
// installation. Only HCEnc and its notices are extracted. Nothing is run.
func InstallRecommendedHCenc(ctx context.Context, progress func(DownloadProgress)) (string, error) {
	if runtime.GOOS != "windows" {
		return "", fmt.Errorf("automatic HCEnc setup requires Windows")
	}
	client := &http.Client{Timeout: 5 * time.Minute, CheckRedirect: checkHCencRedirect}
	return installHCenc(ctx, HCencCacheRoot(), client, hcencArchiveURL, hcencArchiveSHA256, hcencExeSHA256, progress)
}

func checkHCencRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= 10 || req.URL.User != nil ||
		(req.URL.Scheme != "http" && req.URL.Scheme != "https") ||
		(!strings.EqualFold(req.URL.Host, "hank315.nl") && !strings.EqualFold(req.URL.Host, "www.hank315.nl")) {
		return fmt.Errorf("unexpected HCEnc download redirect")
	}
	return nil
}

func installHCenc(ctx context.Context, root string, client *http.Client, source, archiveHash, exeHash string, progress func(DownloadProgress)) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if err := os.MkdirAll(root, 0700); err != nil {
		return "", err
	}
	unlock, err := platform.LockFile(filepath.Join(root, "install.lock"))
	if err != nil {
		return "", fmt.Errorf("another HCEnc setup is running: %w", err)
	}
	defer unlock()
	if path := managedHCencWithHash(root, exeHash); path != "" {
		return path, nil
	}
	stage, err := os.MkdirTemp(root, ".install-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(stage)
	emit := func(p DownloadProgress) {
		if progress != nil {
			p.Stage = strings.ReplaceAll(p.Stage, "FFmpeg", "HCEnc")
			progress(p)
		}
	}
	archive := filepath.Join(stage, "HCEnc.zip")
	if err := downloadArchiveLimited(ctx, client, source, archiveHash, archive, emit, 64<<20); err != nil {
		return "", fmt.Errorf("HCEnc setup: %w", err)
	}
	ready := filepath.Join(stage, "ready")
	if err := os.Mkdir(ready, 0700); err != nil {
		return "", err
	}
	if err := extractHCencZIP(archive, ready); err != nil {
		return "", err
	}
	path := filepath.Join(ready, "HCenc.exe")
	if hash, err := HashFile(ctx, path); err != nil || hash != exeHash {
		return "", fmt.Errorf("HCEnc executable checksum mismatch; nothing was installed")
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	final := filepath.Join(root, hcencVersion)
	if _, err := os.Stat(final); !os.IsNotExist(err) {
		final += "-" + strings.TrimPrefix(filepath.Base(stage), ".install-")
	}
	if err := os.Rename(ready, final); err != nil {
		return "", err
	}
	emit(DownloadProgress{Stage: "HCEnc is ready"})
	return filepath.Join(final, "HCenc.exe"), nil
}

func extractHCencZIP(archive, ready string) error {
	z, err := zip.OpenReader(archive)
	if err != nil {
		return err
	}
	defer z.Close()
	wanted := map[string]string{
		"HC028_21-12-2015/HCenc_028.exe":      "HCenc.exe",
		"HC028_21-12-2015/misc/thanks.txt":    "HCenc-Copyright.txt",
		"HC028_21-12-2015/misc/changelog.txt": "HCenc-changelog.txt",
		"HC028_21-12-2015/misc/HC028.pdf":     "HC028.pdf",
	}
	for _, entry := range z.File {
		name, ok := wanted[entry.Name]
		if !ok {
			continue
		}
		if entry.UncompressedSize64 > 10<<20 || entry.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("invalid HCEnc archive entry")
		}
		in, err := entry.Open()
		if err != nil {
			return err
		}
		data, readErr := io.ReadAll(io.LimitReader(in, (10<<20)+1))
		in.Close()
		if readErr != nil {
			return readErr
		}
		if len(data) > 10<<20 {
			return fmt.Errorf("HCEnc archive entry exceeds size limit")
		}
		if err := os.WriteFile(filepath.Join(ready, name), data, 0600); err != nil {
			return err
		}
		delete(wanted, entry.Name)
	}
	if len(wanted) != 0 {
		return fmt.Errorf("HCEnc archive is incomplete")
	}
	return nil
}
