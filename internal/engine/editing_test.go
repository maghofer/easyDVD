package engine

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"easydvd/internal/media"
	"easydvd/internal/model"
	"easydvd/internal/store"
)

func editingFixture(t *testing.T) (*Engine, *store.Store) {
	t.Helper()
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "queue.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	s := model.DefaultSettings(dir)
	var jobs []model.Job
	for n := 0; n < 4; n++ {
		j := model.Job{ID: model.ID(), Title: "Collection", State: model.Ready, ScanComplete: true, Settings: s}
		for _, id := range []string{"a", "b", "c", "d"} {
			j.Videos = append(j.Videos, model.Video{ID: id, Relative: id, Label: id, Duration: 30})
		}
		jobs = append(jobs, j)
	}
	if err = st.Save(jobs); err != nil {
		t.Fatal(err)
	}
	e, err := New(st, s, media.Tools{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(e.Close)
	return e, st
}

func TestBatchSettingsAreAtomicAndKeepMixedValues(t *testing.T) {
	e, _ := editingFixture(t)
	snap := e.Snapshot()
	ids := []string{snap.Jobs[0].ID, snap.Jobs[1].ID}
	e.mu.Lock()
	e.jobs[1].Settings.Standard = "NTSC"
	e.mu.Unlock()
	if err := e.UpdateJobs(ids, func(s *model.Settings) { s.Media = "DVD-9" }); err != nil {
		t.Fatal(err)
	}
	got := e.Snapshot()
	if got.Jobs[0].Settings.Standard != "PAL" || got.Jobs[1].Settings.Standard != "NTSC" || got.Jobs[1].Settings.Media != "DVD-9" {
		t.Fatal(got)
	}
	e.mu.Lock()
	e.jobs[1].Locked = true
	e.mu.Unlock()
	before := e.Snapshot().Jobs
	if err := e.UpdateJobs(ids, func(s *model.Settings) { s.EncodingProfile = model.ProfileQuality }); err == nil {
		t.Fatal("locked batch accepted")
	}
	if !reflect.DeepEqual(before, e.Snapshot().Jobs) {
		t.Fatal("partially applied rejected batch")
	}
}

func TestClearRenderStatusPreservesFilesAndInvalidatesArtifacts(t *testing.T) {
	e, st := editingFixture(t)
	output := filepath.Join(t.TempDir(), "old.iso")
	if err := os.WriteFile(output, []byte("previous ISO"), 0600); err != nil {
		t.Fatal(err)
	}
	e.mu.Lock()
	j := &e.jobs[0]
	j.State = model.Complete
	j.Locked = true
	j.OutputPath = output
	j.Progress = 1
	j.Generation = "old"
	j.ElapsedSeconds = 123
	j.Videos[0].Encoded = output
	j.Videos[0].EncodedHash = "oldhash"
	j.Videos[0].EncodingKey = "oldkey"
	j.Videos[0].EncodedSize = 12
	j.Videos[0].Label = "Custom title"
	j.Videos[1].Excluded = true
	id := j.ID
	e.mu.Unlock()
	if err := e.ClearRenderStatus([]string{id}); err != nil {
		t.Fatal(err)
	}
	got := e.Snapshot().Jobs[0]
	if got.RenderRevision != 1 || got.Locked || got.OutputPath != "" || got.Progress != 0 || got.ElapsedSeconds != 0 || got.State != model.Ready || got.Videos[0].Encoded != "" || got.Videos[0].EncodingKey != "" || got.Videos[0].Label != "Custom title" || !got.Videos[1].Excluded {
		t.Fatal(got)
	}
	b, err := os.ReadFile(output)
	if err != nil || string(b) != "previous ISO" {
		t.Fatal("previous output changed", err)
	}
	saved, err := st.Load()
	if err != nil || saved[0].RenderRevision != 1 {
		t.Fatal("reset not durable", err)
	}
	if err := e.ClearRenderStatus([]string{id}); err == nil {
		t.Fatal("pending job reset accepted")
	}
}

func TestManualOrderMovesBlocksAndSurvivesRescanSorting(t *testing.T) {
	e, st := editingFixture(t)
	snap := e.Snapshot()
	id := snap.Jobs[0].ID
	if err := e.ReorderVideos(id, []string{"b", "c"}, 4); err != nil {
		t.Fatal(err)
	}
	j := e.Snapshot().Jobs[0]
	want := []string{"a", "d", "b", "c"}
	if !reflect.DeepEqual(j.VideoOrder, want) {
		t.Fatal(j.VideoOrder)
	}
	j.Videos[0], j.Videos[3] = j.Videos[3], j.Videos[0]
	applyVideoOrder(&j)
	for i, v := range j.Videos {
		if v.ID != want[i] {
			t.Fatal(j.Videos)
		}
	}
	if err := e.ReorderJobs([]string{snap.Jobs[1].ID, snap.Jobs[2].ID}, 4); err != nil {
		t.Fatal(err)
	}
	loaded, err := st.Load()
	if err != nil || loaded[1].ID != snap.Jobs[3].ID || loaded[2].ID != snap.Jobs[1].ID || loaded[3].ID != snap.Jobs[2].ID {
		t.Fatal(loaded, err)
	}
}

func TestClearingFailedProbeRequestsFreshInspection(t *testing.T) {
	e, _ := editingFixture(t)
	e.mu.Lock()
	j := &e.jobs[0]
	id := j.ID
	j.State = model.Blocked
	j.Videos[0].Duration = 0
	j.Videos[0].Error = "file inspection failed"
	e.mu.Unlock()
	if err := e.ClearRenderStatus([]string{id}); err != nil {
		t.Fatal(err)
	}
	got := e.Snapshot().Jobs[0]
	if got.State != model.Paused || got.ScanComplete {
		t.Fatal("reset would remain blocked instead of rescanning on Start", got)
	}
}
