package ui

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"easydvd/internal/model"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

func jobFixture(t *testing.T) *window {
	return fileListFixture(t,
		model.Job{Title: "Holiday 2002", State: model.Ready, ScanComplete: true},
		model.Job{Title: "Family memories", State: model.Ready, ScanComplete: true},
		model.Job{Title: "Winter 2003", State: model.Complete, ScanComplete: true, Progress: 1},
		model.Job{Title: "Weekend", State: model.Ready, ScanComplete: true})
}

func waitForJobCount(t *testing.T, u *window, count int) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		s := u.engine.Snapshot()
		if len(s.Jobs) == count && !s.Clearing {
			u.snapshot = s
			u.refresh()
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("job removal did not finish")
}

func TestJobSelectionRangesAndDelete(t *testing.T) {
	u := jobFixture(t)
	jobs := append([]model.Job(nil), u.snapshot.Jobs...)
	u.selectJobs(jobs[1].ID, 0)
	u.selectJobs(jobs[3].ID, fyne.KeyModifierShift)
	if len(u.selectedJobs) != 3 || u.selected != jobs[3].ID || u.jobSelectionCount.Text != "3 selected" {
		t.Fatal("job Shift-selection differs from file selection")
	}
	u.selectJobs(jobs[0].ID, fyne.KeyModifierControl)
	if len(u.selectedJobs) != 4 {
		t.Fatal("Ctrl-click discarded previous selection")
	}
	u.selectJobs(jobs[0].ID, fyne.KeyModifierControl)
	if len(u.selectedJobs) != 3 || u.remove.Disabled() || u.remove.Text != "Remove selected" {
		t.Fatal("job toggling or bulk removal control is incorrect")
	}
	if u.win.Canvas().Focused() != u.queue {
		t.Fatal("job click did not focus the queue for keyboard actions")
	}
	u.win.Canvas().Focused().TypedKey(&fyne.KeyEvent{Name: fyne.KeyDelete})
	waitForJobCount(t, u, 2)
	if u.snapshot.Jobs[0].ID != jobs[0].ID || u.snapshot.Jobs[1].ID != jobs[4].ID || len(u.selectedJobs) != 0 {
		t.Fatal("Delete removed jobs outside the selection or kept stale selections")
	}
	test.Tap(u.jobSelectAll)
	if len(u.selectedJobs) != 2 {
		t.Fatal("Select all missed queued jobs")
	}
	test.Tap(u.remove)
	waitForJobCount(t, u, 0)
	if !u.empty.Visible() || !u.remove.Disabled() {
		t.Fatal("removing all selected jobs did not restore the empty state")
	}
}

func TestJobTileCloseTargetsOnlyItsJob(t *testing.T) {
	u := jobFixture(t)
	keep := u.snapshot.Jobs[0].ID
	u.selectJobs(keep, 0)
	r := newJobTile(u)
	r.update(u.snapshot.Jobs[1])
	if r.MinSize().Height > 48 {
		t.Fatalf("job tile still has too much vertical space: %v", r.MinSize())
	}
	test.Tap(r.close)
	waitForJobCount(t, u, 4)
	if u.snapshot.Jobs[0].ID != keep || !u.selectedJobs[keep] {
		t.Fatal("tile X removed a selected job instead of its own job")
	}
	// A recycled tile must bind its X to the next displayed job.
	r.update(u.snapshot.Jobs[2])
	id := r.id
	test.Tap(r.close)
	waitForJobCount(t, u, 3)
	for _, job := range u.snapshot.Jobs {
		if job.ID == id {
			t.Fatal("recycled X targeted the old job")
		}
	}
}

func TestJobStatusAndSharedFooter(t *testing.T) {
	u := jobFixture(t)
	for _, tc := range []struct {
		job   model.Job
		label string
	}{
		{model.Job{State: model.Ready}, "Ready"},
		{model.Job{State: model.Running}, "Running"},
		{model.Job{State: model.Complete}, "Completed"},
		{model.Job{State: model.Blocked, Locked: true}, "Failed"},
		{model.Job{State: model.Blocked, Videos: []model.Video{{Error: "Unreadable file"}}}, "Needs attention"},
	} {
		label, _ := jobAppearance(tc.job)
		if label != tc.label {
			t.Fatalf("status %s displayed as %s", tc.label, label)
		}
	}
	u.snapshot.Jobs[2].State = model.Running
	u.snapshot.Jobs[2].Progress = .426
	u.snapshot.Jobs[2].Stage = "Encoding · 4 of 10 complete"
	u.selected = u.snapshot.Jobs[0].ID
	u.refresh()
	if !u.footerProgress.Visible() || u.queuePosition.Text != "Job 3 of 5" || u.queuePercent.Text != "43%" || u.queueProgress.Size().Height != 6 {
		t.Fatal("shared status bar does not track the running job independently of the viewed job")
	}
	r := newJobTile(u)
	r.update(u.snapshot.Jobs[2])
	if !strings.Contains(r.state.Text, "43%") {
		t.Fatal("running job tile has no percentage")
	}
}

func TestQueuePreview(t *testing.T) {
	dir := os.Getenv("EASYDVD_UI_PREVIEW")
	if dir == "" {
		t.Skip("visual preview not requested")
	}
	u := jobFixture(t)
	base := u.snapshot.Jobs[1].Clone()
	for i := len(u.snapshot.Jobs); i < 14; i++ {
		j := base.Clone()
		j.ID, j.Title = model.ID(), fmt.Sprintf("Family films %02d", i+1)
		u.snapshot.Jobs = append(u.snapshot.Jobs, j)
	}
	u.snapshot.Jobs[1].State, u.snapshot.Jobs[1].Progress = model.Running, .42
	u.snapshot.Jobs[1].Stage = "Encoding · 4 of 10 complete"
	u.snapshot.Jobs[2].State, u.snapshot.Jobs[2].Locked = model.Blocked, true
	u.snapshot.Jobs[2].Error = "Encoding failed"
	u.selectedJobs[u.snapshot.Jobs[4].ID] = true
	for _, appearance := range []string{"Light", "Dark"} {
		u.applyTheme(appearance)
		u.refresh()
		f, err := os.Create(filepath.Join(dir, "queue-"+appearance+".png"))
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, u.win.Canvas().Capture()); err != nil {
			t.Fatal(err)
		}
		f.Close()
	}
}
