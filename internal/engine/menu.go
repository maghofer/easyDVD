package engine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"easydvd/internal/fsops"
	"easydvd/internal/media"
	"easydvd/internal/menu"
	"easydvd/internal/model"
)

func (e *Engine) MenuPresets() ([]model.MenuPreset, error) { return e.store.MenuPresets() }
func (e *Engine) SaveMenuPreset(p model.MenuPreset) error  { return e.store.SaveMenuPreset(p) }
func (e *Engine) SetMenu(id string, style model.MenuStyle, layout string, count int) error {
	s := e.Snapshot().Settings
	if id != "" {
		found := false
		for _, j := range e.Snapshot().Jobs {
			if j.ID == id {
				s = j.Settings
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("job not found")
		}
	}
	s.Menu = style
	s.Layout = layout
	s.TitlesPerPage = count
	if id != "" {
		s.NoMenu = false
		return e.SetJobSettings(id, s)
	}
	return e.SetSettings(id, s)
}

// Import immutable assets so presets and queued jobs survive source removal.
func (e *Engine) ImportMenuAsset(ctx context.Context, path, kind string) (string, error) {
	limit := int64(32 << 20)
	if kind == "font" {
		limit = 20 << 20
	}
	info, err := fsops.Stat(ctx, path)
	if err != nil {
		return "", err
	}
	if !info.Exists || info.Directory || info.Size > limit {
		return "", fmt.Errorf("choose an image up to 32 MB or font up to 20 MB")
	}
	work := e.Snapshot().Settings.WorkDir
	if err = os.MkdirAll(work, 0755); err != nil {
		return "", err
	}
	stage, err := os.MkdirTemp(work, "menu-import-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(stage)
	local := filepath.Join(stage, "asset"+strings.ToLower(filepath.Ext(path)))
	if err = fsops.Copy(ctx, path, local); err != nil {
		return "", err
	}
	if err = menu.ValidateAsset(local, kind); err != nil {
		return "", err
	}
	hash, err := media.HashFile(ctx, local)
	if err != nil {
		return "", err
	}
	dir := filepath.Join(e.dataDir, "menu-assets")
	if err = os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	dest := filepath.Join(dir, hash+strings.ToLower(filepath.Ext(path)))
	if _, err = os.Stat(dest); err == nil {
		return dest, nil
	}
	temp, err := os.CreateTemp(dir, "import-*.partial")
	if err != nil {
		return "", err
	}
	temporary := temp.Name()
	temp.Close()
	defer os.Remove(temporary)
	if err = CopyFile(ctx, local, temporary); err != nil {
		return "", err
	}
	if err = os.Rename(temporary, dest); err != nil {
		return "", err
	}
	return dest, nil
}
