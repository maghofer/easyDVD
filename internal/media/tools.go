package media

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"easydvd/internal/process"
)

type Tools struct {
	FFmpeg, FFprobe, DVDAuthor, Spumux, Mkisofs string
	FFmpegIssue                                 string
}
type ToolStatus struct{ Name, Path, Version, Error string }
type safeBuffer struct {
	mu sync.Mutex
	b  strings.Builder
}

func (b *safeBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.Write(p)
}
func (b *safeBuffer) String() string { b.mu.Lock(); defer b.mu.Unlock(); return b.b.String() }

func FindAuthoringTools() Tools {
	exe, _ := os.Executable()
	cwd, _ := os.Getwd()
	roots := []string{os.Getenv("EASYDVD_TOOLS"), filepath.Join(filepath.Dir(exe), "tools"), filepath.Join(cwd, ".tools", "media", "bin")}
	find := func(name string) string {
		suffix := ""
		if runtime.GOOS == "windows" {
			suffix = ".exe"
		}
		for _, root := range roots {
			if root != "" {
				p := filepath.Join(root, name+suffix)
				if st, e := os.Stat(p); e == nil && !st.IsDir() {
					return p
				}
			}
		}
		if p, e := exec.LookPath(name); e == nil {
			return p
		}
		return ""
	}
	return Tools{DVDAuthor: find("dvdauthor"), Spumux: find("spumux"), Mkisofs: find("mkisofs")}
}

func FindTools() Tools {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	t, _ := ResolveTools(ctx, "")
	return t
}
func (t Tools) Check(ctx context.Context) []ToolStatus {
	var out []ToolStatus
	for _, v := range []struct {
		name, path string
		args       []string
	}{{"FFmpeg", t.FFmpeg, []string{"-version"}}, {"FFprobe", t.FFprobe, []string{"-version"}}, {"dvdauthor", t.DVDAuthor, []string{"-h"}}, {"spumux", t.Spumux, []string{"-h"}}, {"mkisofs", t.Mkisofs, []string{"-version"}}} {
		s := ToolStatus{Name: v.name, Path: v.path}
		if v.path == "" {
			s.Error = "Not found"
			if (v.name == "FFmpeg" || v.name == "FFprobe") && t.FFmpegIssue != "" {
				s.Error = t.FFmpegIssue
			}
		} else {
			var b safeBuffer
			err := process.Run(ctx, v.path, v.args, process.Options{Stdout: &b, Log: &b})
			if err != nil {
				s.Error = err.Error()
			}
			s.Version = strings.TrimSpace(b.String())
			version := regexp.MustCompile(`(?m)^(ffmpeg version|ffprobe version|DVDAuthor::|mkisofs )[^\r\n]*`).FindString(s.Version)
			if version != "" {
				s.Version = version
				if (v.name == "spumux" || v.name == "dvdauthor") && ctx.Err() == nil {
					s.Error = ""
				}
			}
		}
		out = append(out, s)
	}
	return out
}
func (t Tools) CheckAuthoring(ctx context.Context) []ToolStatus {
	all := t.Check(ctx)
	var authors []ToolStatus
	for _, s := range all {
		if s.Name != "FFmpeg" && s.Name != "FFprobe" {
			authors = append(authors, s)
		}
	}
	return authors
}
func (t Tools) Require() error {
	if t.FFmpeg == "" || t.FFprobe == "" {
		reason := t.FFmpegIssue
		if reason == "" {
			reason = "a compatible FFmpeg / FFprobe pair was not found"
		}
		return fmt.Errorf("FFmpeg setup required: %s. Use FFmpeg setup in the app, -download-ffmpeg, or EASYDVD_FFMPEG", reason)
	}
	return t.RequireAuthoring()
}
func (t Tools) RequireAuthoring() error {
	for _, v := range []struct{ name, path string }{{"dvdauthor", t.DVDAuthor}, {"spumux", t.Spumux}, {"mkisofs", t.Mkisofs}} {
		if v.path == "" {
			return fmt.Errorf("bundled authoring tool %s is missing; re-extract the application package or set EASYDVD_TOOLS", v.name)
		}
	}
	return nil
}
