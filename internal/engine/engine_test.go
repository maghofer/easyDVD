package engine

import (
	"context"
	"easydvd/internal/media"
	"easydvd/internal/model"
	"easydvd/internal/store"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRecoverAndLockSettings(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "q.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	settings := model.DefaultSettings(t.TempDir())
	id := model.ID()
	if err = st.Save([]model.Job{{ID: id, State: model.Running, Locked: true, ScanComplete: true, Settings: settings}}); err != nil {
		t.Fatal(err)
	}
	e, err := New(st, settings, media.Tools{})
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	s := e.Snapshot()
	if s.Jobs[0].State != model.Paused || s.Enabled {
		t.Fatal(s)
	}
	next := settings
	next.Standard = "NTSC"
	if e.SetSettings(id, next) == nil {
		t.Fatal("started settings changed")
	}
	if err = e.SetSettings("", next); err != nil {
		t.Fatal(err)
	}
	if e.Snapshot().Jobs[0].Settings.Standard != "PAL" {
		t.Fatal("global settings changed a locked job")
	}
}
func TestCopyCancellation(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	if err := os.WriteFile(source, []byte("valuable source"), 0644); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := CopyFile(ctx, source, filepath.Join(dir, "out")); err != context.Canceled {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(source)
	if string(b) != "valuable source" {
		t.Fatal("source changed")
	}
}

func TestBulkExclusionIsAtomicAndPersisted(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "q.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	s := model.DefaultSettings(t.TempDir())
	j := model.Job{ID: model.ID(), State: model.Ready, ScanComplete: true, Settings: s, Videos: []model.Video{
		{ID: "one", Label: "Custom title", Duration: 30, Encoded: "cached.mpg"},
		{ID: "two", Label: "Second", Duration: 40},
		{ID: "three", Label: "Third", Duration: 50},
	}}
	if err := st.Save([]model.Job{j}); err != nil {
		t.Fatal(err)
	}
	e, err := New(st, s, media.Tools{})
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	if err := e.SetExcluded(j.ID, []string{"one", "missing"}, true); err == nil {
		t.Fatal("stale selection was accepted")
	}
	if e.Snapshot().Jobs[0].Videos[0].Excluded {
		t.Fatal("stale selection was partially applied")
	}
	if err := e.SetExcluded(j.ID, []string{"one", "two"}, true); err != nil {
		t.Fatal(err)
	}
	saved, err := st.Load()
	if err != nil {
		t.Fatal(err)
	}
	v := saved[0].Videos
	if !v[0].Excluded || !v[1].Excluded || v[2].Excluded || v[0].Label != "Custom title" || v[0].Encoded != "cached.mpg" {
		t.Fatal("bulk exclusion was not persisted correctly, or changed unrelated data")
	}
	if e.Snapshot().Jobs[0].Duration() != 50 {
		t.Fatal("excluded files still contribute to job duration")
	}
	if err := e.SetExcluded(j.ID, []string{"one", "two"}, false); err != nil {
		t.Fatal(err)
	}
	for _, state := range []model.State{model.Running, model.Scanning, model.Complete} {
		e.mu.Lock()
		e.jobs[0].State = state
		e.mu.Unlock()
		if err := e.SetExcluded(j.ID, []string{"one", "two"}, true); err == nil {
			t.Fatalf("bulk edits allowed in state %s", state)
		}
		if e.Snapshot().Jobs[0].Videos[0].Excluded {
			t.Fatal("rejected bulk edit mutated the job")
		}
	}
}

func TestClearQueueStopsWorkersAndKeepsFiles(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "q.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	s := model.DefaultSettings(t.TempDir())
	jobs := []model.Job{
		{ID: model.ID(), State: model.Ready, Settings: s},
		{ID: model.ID(), State: model.Paused, Settings: s},
		{ID: model.ID(), State: model.Complete, Settings: s},
		{ID: model.ID(), State: model.Blocked, Settings: s},
	}
	// These files must survive clearing queue records, including a completed ISO.
	var keep []string
	for _, name := range []string{"source.mp4", "encoded.mpg", "completed.iso"} {
		path := filepath.Join(t.TempDir(), name)
		if err := os.WriteFile(path, []byte("keep this file"), 0644); err != nil {
			t.Fatal(err)
		}
		keep = append(keep, path)
	}
	jobs[0].Videos = []model.Video{{ID: "video", Path: keep[0], Encoded: keep[1]}}
	jobs[2].OutputPath = filepath.Dir(keep[2])
	jobs[2].Discs = []model.Disc{{ISO: keep[2]}}
	if err := st.Save(jobs); err != nil {
		t.Fatal(err)
	}
	e, err := New(st, s, media.Tools{})
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	processing, stopProcessing := context.WithCancel(context.Background())
	defer stopProcessing()
	scanning, stopScanning := context.WithCancel(context.Background())
	defer stopScanning()
	// Controlled worker lifetimes let us prove removal waits for both workers.
	e.mu.Lock()
	e.jobs[0].State = model.Running
	e.jobs[1].State = model.Scanning
	e.cancels[jobs[0].ID] = stopProcessing
	e.scans[jobs[1].ID] = stopScanning
	e.mu.Unlock()
	done := make(chan error, 1)
	go func() { done <- e.ClearQueue() }()
	for _, ctx := range []context.Context{processing, scanning} {
		select {
		case <-ctx.Done():
		case <-time.After(3 * time.Second):
			t.Fatal("clear queue did not cancel active work")
		}
	}
	if snapshot := e.Snapshot(); !snapshot.Clearing || len(snapshot.Jobs) != 4 {
		t.Fatal("jobs were removed before their workers stopped")
	}
	e.Start()
	e.Retry(jobs[3].ID)
	if e.Snapshot().Enabled {
		t.Fatal("processing can restart during clear queue")
	}
	select {
	case <-done:
		t.Fatal("clear queue returned before workers stopped")
	default:
	}
	// A folder explicitly added during cancellation belongs to the next queue.
	newFolder := t.TempDir()
	if err := e.Add(newFolder); err != nil {
		t.Fatal(err)
	}
	e.mu.Lock()
	delete(e.cancels, jobs[0].ID)
	delete(e.scans, jobs[1].ID)
	e.mu.Unlock()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("clear queue did not finish after workers stopped")
	}
	snapshot := e.Snapshot()
	if snapshot.Clearing || snapshot.Enabled || len(snapshot.Jobs) != 1 || snapshot.Jobs[0].Folder != newFolder {
		t.Fatal("clear queue did not remove exactly the original jobs")
	}
	saved, err := st.Load()
	if err != nil || len(saved) != 1 || saved[0].Folder != newFolder {
		t.Fatal("cleared queue was not persisted", err)
	}
	for _, path := range keep {
		if data, err := os.ReadFile(path); err != nil || string(data) != "keep this file" {
			t.Fatal("clear queue changed a source or output file", path, err)
		}
	}
	if err := e.ClearQueue(); err != nil {
		t.Fatal(err)
	}
	if len(e.Snapshot().Jobs) != 0 {
		t.Fatal("second clear did not empty the queue")
	}
}
