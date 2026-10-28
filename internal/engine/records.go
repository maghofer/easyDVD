package engine

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"easydvd/internal/model"
)

func (e *Engine) LogsDir() string             { return filepath.Join(e.dataDir, "logs") }
func (e *Engine) JobLogPath(id string) string { return filepath.Join(e.LogsDir(), id+".log") }
func (e *Engine) ManifestPath(id string) string {
	return filepath.Join(e.dataDir, "jobs", id, "manifest.json")
}
func PreviewDir(j model.Job) string {
	return filepath.Join(j.Settings.WorkDir, "previews", j.ID, j.Generation)
}

func (e *Engine) saveManifest(j model.Job) error {
	path := e.ManifestPath(j.ID)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(j, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), "manifest-*.partial")
	if err != nil {
		return err
	}
	temp := f.Name()
	defer os.Remove(temp)
	if _, err = f.Write(data); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(temp, path)
}

func (e *Engine) prepareLog(ctx context.Context, j model.Job) error {
	if err := os.MkdirAll(e.LogsDir(), 0755); err != nil {
		return err
	}
	if _, err := os.Stat(e.JobLogPath(j.ID)); !os.IsNotExist(err) {
		return err
	}
	// Preserve diagnostic history when resuming work written by earlier versions.
	legacy := filepath.Join(j.Settings.WorkDir, "jobs", j.ID, "job.log")
	if _, err := os.Stat(legacy); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	return CopyFile(ctx, legacy, e.JobLogPath(j.ID))
}
