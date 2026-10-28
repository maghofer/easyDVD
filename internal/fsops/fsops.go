// Package fsops isolates potentially blocking network filesystem operations.
package fsops

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"easydvd/internal/platform"
	"easydvd/internal/process"
)

type Info struct {
	Exists    bool
	Directory bool
	Size      int64
	ModTime   int64
}

func invoke(ctx context.Context, args []string, result any) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	var b bytes.Buffer
	err = process.Run(ctx, exe, append([]string{"--fs-worker"}, args...), process.Options{Stdout: &b})
	if err != nil {
		return err
	}
	if result != nil {
		return json.Unmarshal(b.Bytes(), result)
	}
	return nil
}
func Stat(ctx context.Context, path string) (Info, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	var info Info
	err := invoke(ctx, []string{"stat", path}, &info)
	return info, err
}
func Copy(ctx context.Context, src, dst string) error {
	return invoke(ctx, []string{"copy", src, dst}, nil)
}
func Hash(ctx context.Context, path string) (string, error) {
	var h string
	err := invoke(ctx, []string{"hash", path}, &h)
	return h, err
}
func Free(ctx context.Context, path string) (uint64, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	var free uint64
	err := invoke(ctx, []string{"space", path}, &free)
	return free, err
}
func Mkdir(ctx context.Context, path string) error { return invoke(ctx, []string{"mkdir", path}, nil) }
func ResetPartial(ctx context.Context, path, id string) error {
	return invoke(ctx, []string{"reset-partial", path, id}, nil)
}
func RequireLocal(ctx context.Context, path string) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	var local bool
	if err := invoke(ctx, []string{"local", path}, &local); err != nil {
		return err
	}
	if !local {
		return fmt.Errorf("choose a local work directory; network drives may be used for sources and outputs")
	}
	return nil
}
func Rename(ctx context.Context, src, dst string) error {
	return invoke(ctx, []string{"rename", src, dst}, nil)
}

func Worker(args []string, out io.Writer) error {
	if len(args) < 2 {
		return fmt.Errorf("invalid filesystem worker operation")
	}
	var result any
	switch args[0] {
	case "local":
		result = platform.IsLocal(args[1])
	case "stat":
		st, err := os.Stat(args[1])
		if os.IsNotExist(err) {
			result = Info{}
		} else if err != nil {
			return err
		} else {
			result = Info{true, st.IsDir(), st.Size(), st.ModTime().UnixNano()}
		}
	case "copy":
		if len(args) != 3 {
			return fmt.Errorf("copy needs source and destination")
		}
		in, err := os.Open(args[1])
		if err != nil {
			return err
		}
		defer in.Close()
		if err = os.MkdirAll(filepath.Dir(args[2]), 0755); err != nil {
			return err
		}
		f, err := os.Create(args[2])
		if err != nil {
			return err
		}
		_, err = io.CopyBuffer(f, in, make([]byte, 1024*1024))
		if err == nil {
			err = f.Sync()
		}
		closeErr := f.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
	case "hash":
		f, err := os.Open(args[1])
		if err != nil {
			return err
		}
		defer f.Close()
		h := sha256.New()
		if _, err = io.CopyBuffer(h, f, make([]byte, 1024*1024)); err != nil {
			return err
		}
		result = hex.EncodeToString(h.Sum(nil))
	case "space":
		free, err := platform.FreeBytes(args[1])
		if err != nil {
			return err
		}
		result = free
	case "mkdir":
		return os.MkdirAll(args[1], 0755)
	case "reset-partial":
		if len(args) != 3 || len(args[2]) != 24 || strings.Trim(args[2], "0123456789abcdef") != "" || !filepath.IsAbs(args[1]) || filepath.Base(args[1]) != ".easydvd-"+args[2]+".partial" {
			return fmt.Errorf("invalid private output staging directory")
		}
		if err := os.RemoveAll(args[1]); err != nil {
			return err
		}
		return os.MkdirAll(args[1], 0755)
	case "rename":
		if len(args) != 3 {
			return fmt.Errorf("rename needs source and destination")
		}
		return os.Rename(args[1], args[2])
	default:
		return fmt.Errorf("unknown filesystem operation")
	}
	return json.NewEncoder(out).Encode(result)
}
