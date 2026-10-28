package media

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"easydvd/internal/process"
)

type ScanRecord struct {
	Path  string `json:"path,omitempty"`
	Error string `json:"error,omitempty"`
}

// ScanWorker runs in an isolated process: an unavailable SMB share cannot strand the GUI.
func ScanWorker(root string, exclude []string, out io.Writer) error {
	st, err := os.Stat(root)
	if err != nil {
		return err
	}
	if !st.IsDir() {
		return fmt.Errorf("add folders, not individual files")
	}
	enc := json.NewEncoder(out)
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			_ = enc.Encode(ScanRecord{Error: fmt.Sprintf("%s: %v", path, err)})
			return err
		}
		if d.IsDir() {
			for _, x := range exclude {
				if x != "" && filepath.Clean(path) == filepath.Clean(x) {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 || !IsVideo(path) {
			return nil
		}
		relative, e := filepath.Rel(root, path)
		if e != nil {
			return e
		}
		return enc.Encode(ScanRecord{Path: relative})
	})
}
func Discover(ctx context.Context, root string, exclude []string, found func(string)) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	args := append([]string{"--scan-worker", root}, exclude...)
	var parseErr error
	err = process.Run(ctx, exe, args, process.Options{Line: func(line string) {
		var r ScanRecord
		if e := json.Unmarshal([]byte(line), &r); e != nil {
			parseErr = e
			return
		}
		if r.Error != "" {
			parseErr = fmt.Errorf("%s", r.Error)
		}
		if r.Path != "" {
			found(r.Path)
		}
	}})
	if err != nil {
		return err
	}
	return parseErr
}
