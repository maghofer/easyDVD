package media

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func hcencFixture(t *testing.T, exe []byte, extra string) []byte {
	t.Helper()
	var b bytes.Buffer
	w := zip.NewWriter(&b)
	files := map[string][]byte{
		"HC028_21-12-2015/HCenc_028.exe":      exe,
		"HC028_21-12-2015/misc/thanks.txt":    []byte("copyright fixture"),
		"HC028_21-12-2015/misc/changelog.txt": []byte("changelog fixture"),
		"HC028_21-12-2015/misc/HC028.pdf":     []byte("manual fixture"),
	}
	if extra != "" {
		files[extra] = []byte("must not be extracted")
	}
	for name, data := range files {
		f, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func hcHash(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }

func TestHCencDownloadVerifiesBothHashesAndReusesCache(t *testing.T) {
	exe := []byte("encoder fixture")
	archive := hcencFixture(t, exe, "../escape.txt")
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if !strings.HasPrefix(r.UserAgent(), "easyDVD/") {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.Write(archive)
	}))
	defer server.Close()
	root := filepath.Join(t.TempDir(), "private")
	install := func(archiveHash, exeHash string) (string, error) {
		return installHCenc(context.Background(), root, server.Client(), server.URL, archiveHash, exeHash, nil)
	}
	if _, err := install(strings.Repeat("0", 64), hcHash(exe)); err == nil {
		t.Fatal("bad archive hash accepted")
	}
	if _, err := install(hcHash(archive), strings.Repeat("0", 64)); err == nil {
		t.Fatal("bad executable hash accepted")
	}
	path, err := install(hcHash(archive), hcHash(exe))
	if err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(path); err != nil || !bytes.Equal(data, exe) {
		t.Fatal("encoder was not installed", err)
	}
	if _, err := os.Stat(filepath.Join(root, "escape.txt")); !os.IsNotExist(err) {
		t.Fatal("unlisted ZIP entry escaped extraction")
	}
	before := requests
	again, err := install(hcHash(archive), hcHash(exe))
	if err != nil || again != path || requests != before {
		t.Fatal("verified cache was not reused", err)
	}
	if err := os.WriteFile(path, []byte("corrupted"), 0600); err != nil {
		t.Fatal(err)
	}
	repaired, err := install(hcHash(archive), hcHash(exe))
	if err != nil || repaired == path {
		t.Fatal("corrupted cache was overwritten or reused", err)
	}
	if data, _ := os.ReadFile(path); string(data) != "corrupted" {
		t.Fatal("existing cache file was overwritten")
	}
	entries, _ := os.ReadDir(root)
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".install-") {
			t.Fatal("temporary download survived")
		}
	}
}

func TestHCencRedirectStaysOnPublisherSite(t *testing.T) {
	for _, tc := range []struct {
		url     string
		allowed bool
	}{
		{"http://hank315.nl/files/HC_028/HC028_21-12-2015.zip", true},
		{"https://www.hank315.nl/files/HC_028/HC028_21-12-2015.zip", true},
		{"http://hank315.nl.evil.example/HC.zip", false},
		{"http://elsewhere.example/HC.zip", false},
		{"http://unexpected@hank315.nl/HC.zip", false},
		{"ftp://hank315.nl/HC.zip", false},
	} {
		t.Run(tc.url, func(t *testing.T) {
			r, err := http.NewRequest(http.MethodGet, tc.url, nil)
			if err != nil {
				t.Fatal(err)
			}
			if (checkHCencRedirect(r, nil) == nil) != tc.allowed {
				t.Fatal("unexpected redirect policy result")
			}
			if checkHCencRedirect(r, make([]*http.Request, 10)) == nil {
				t.Fatal("redirect loop accepted")
			}
		})
	}
}

func TestHCencDownloadCancellationAndLimit(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	root := filepath.Join(t.TempDir(), "not-created")
	if _, err := installHCenc(ctx, root, http.DefaultClient, "", "", "", nil); err == nil {
		t.Fatal("cancelled setup succeeded")
	}
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatal("cancelled setup created cache")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Header().Set("Content-Length", "90000000") }))
	defer server.Close()
	if _, err := installHCenc(context.Background(), root, server.Client(), server.URL, "", "", nil); err == nil || !strings.Contains(err.Error(), "size limit") {
		t.Fatal("oversized download was not rejected", err)
	}
}

func TestHCencUnavailableServerLeavesNoPartialInstall(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()
	root := filepath.Join(t.TempDir(), "private")
	if _, err := installHCenc(context.Background(), root, server.Client(), server.URL, hcencArchiveSHA256, hcencExeSHA256, nil); err == nil {
		t.Fatal("unavailable server was treated as a successful setup")
	}
	if managedHCencAt(root) != "" {
		t.Fatal("failed download left an installed encoder")
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Name() != "install.lock" {
			t.Fatalf("failed download left %s", entry.Name())
		}
	}
}
