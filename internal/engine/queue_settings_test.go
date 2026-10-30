package engine

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"easydvd/internal/media"
	"easydvd/internal/model"
	"easydvd/internal/store"
)

func TestSavedDefaultsOnlyAffectNewJobs(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "q.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	s := model.DefaultSettings(t.TempDir())
	custom := s
	custom.Media, custom.OutputDir = "DVD-9", t.TempDir()
	jobs := []model.Job{{ID: model.ID(), State: model.Ready, Settings: s}, {ID: model.ID(), State: model.Ready, Settings: custom}}
	if err := st.Save(jobs); err != nil {
		t.Fatal(err)
	}
	e, err := New(st, s, media.Tools{})
	if err != nil {
		t.Fatal(err)
	}
	next := s
	next.Standard, next.EncodingProfile, next.TitlesPerPage = "NTSC", model.ProfileQuality, 6
	if err := e.SetSettings("", next); err != nil {
		t.Fatal(err)
	}
	if err := e.Add(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	snapshot := e.Snapshot()
	if snapshot.Jobs[0].Settings != s || snapshot.Jobs[1].Settings != custom || snapshot.Jobs[2].Settings != next {
		t.Fatal("saved defaults changed existing jobs or were not inherited by a new job")
	}
	e.Close()
	e, err = New(st, s, media.Tools{})
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	snapshot = e.Snapshot()
	if snapshot.Settings != next || snapshot.Jobs[0].Settings != s || snapshot.Jobs[1].Settings != custom {
		t.Fatal("defaults and overrides did not survive restart independently")
	}
}

func TestRemoveSelectedJobsWaitsOnlyForTheirWorkers(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "q.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	s := model.DefaultSettings(t.TempDir())
	jobs := []model.Job{{ID: model.ID(), State: model.Ready, Settings: s}, {ID: model.ID(), State: model.Ready, Settings: s}, {ID: model.ID(), State: model.Complete, Settings: s}}
	if err := st.Save(jobs); err != nil {
		t.Fatal(err)
	}
	e, err := New(st, s, media.Tools{})
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	if err := e.RemoveJobs([]string{jobs[0].ID, "missing"}); err == nil || len(e.Snapshot().Jobs) != 3 {
		t.Fatal("stale selection was partially removed")
	}
	selected, cancelSelected := context.WithCancel(context.Background())
	defer cancelSelected()
	other, cancelOther := context.WithCancel(context.Background())
	defer cancelOther()
	e.mu.Lock()
	e.jobs[0].State = model.Running
	e.cancels[jobs[0].ID] = cancelSelected
	e.scans[jobs[1].ID] = cancelOther
	e.mu.Unlock()
	done := make(chan error, 1)
	go func() { done <- e.RemoveJobs([]string{jobs[0].ID, jobs[2].ID}) }()
	select {
	case <-selected.Done():
	case <-time.After(3 * time.Second):
		t.Fatal("selected worker was not canceled")
	}
	if other.Err() != nil || len(e.Snapshot().Jobs) != 3 {
		t.Fatal("removal canceled an unrelated worker or removed an active job too early")
	}
	select {
	case <-done:
		t.Fatal("removal did not wait for selected worker")
	default:
	}
	e.mu.Lock()
	delete(e.cancels, jobs[0].ID)
	e.mu.Unlock()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("removal waited for an unrelated worker")
	}
	saved, err := st.Load()
	if err != nil || len(saved) != 1 || saved[0].ID != jobs[1].ID || other.Err() != nil {
		t.Fatal("selected removal was not persisted correctly", err)
	}
}
