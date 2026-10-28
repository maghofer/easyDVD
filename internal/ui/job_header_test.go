package ui

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"easydvd/internal/model"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

func TestJobHeaderCollapsedAndExpandable(t *testing.T) {
	u := fileListFixture(t)
	if u.headerExpanded || u.headerInfo.Visible() || u.progress.Visible() || u.scanProgress.Visible() || u.activity.Visible() {
		t.Fatal("ready job header does not start collapsed without progress")
	}
	collapsed := u.jobHeader.MinSize().Height
	if collapsed > 100 || u.files.Size().Height < 300 {
		t.Fatalf("collapsed header consumes too much space: header=%v list=%v", u.jobHeader.MinSize(), u.files.Size())
	}
	if !u.jobWarning.Visible() || u.jobWarning.Text != "1 file issue" {
		t.Fatal("collapsing the header hid the file issue")
	}
	test.Tap(u.headerToggle)
	if !u.headerExpanded || !u.headerInfo.Visible() || u.jobHeader.MinSize().Height <= collapsed+100 || u.source.Text != u.selectedJob().Folder {
		t.Fatal("expanded job information is missing")
	}
	u.refresh()
	if !u.headerExpanded {
		t.Fatal("routine refresh collapsed the open header")
	}
	test.Tap(u.headerToggle)
	if u.headerExpanded || u.headerInfo.Visible() || u.jobHeader.Size().Height > 100 || u.files.Size().Height < 300 {
		t.Fatal("header cannot collapse again")
	}
	test.Tap(u.headerToggle)
	other := u.snapshot.Jobs[0].Clone()
	other.ID = "another-job"
	u.snapshot.Jobs = append(u.snapshot.Jobs, other)
	u.selected = other.ID
	u.refresh()
	if u.headerExpanded || u.headerInfo.Visible() {
		t.Fatal("a newly selected job did not default to collapsed")
	}
}

func TestJobHeaderProgressFollowsActivity(t *testing.T) {
	u := fileListFixture(t)
	for _, tc := range []struct {
		state                   model.State
		progress                float64
		bar, scanning, activity bool
	}{
		{model.Scanning, 0, false, true, true},
		{model.Ready, 0, false, false, false},
		{model.Running, 0, true, false, true},
		{model.Running, .45, true, false, true},
		{model.Paused, .45, true, false, false},
		{model.Paused, 0, false, false, false},
		{model.Blocked, 0, false, false, false},
		{model.Canceled, .3, false, false, false},
		{model.Complete, 1, false, false, false},
	} {
		u.snapshot.Jobs[0].State = tc.state
		u.snapshot.Jobs[0].Progress = tc.progress
		u.refresh()
		if u.progress.Visible() != tc.bar || u.scanProgress.Visible() != tc.scanning || u.scanProgress.Running() != tc.scanning || u.activity.Visible() != tc.activity {
			t.Fatalf("incorrect progress visibility for %s at %v", tc.state, tc.progress)
		}
	}
}

func TestClearQueueConfirmationAndEmptyState(t *testing.T) {
	u := fileListFixture(t)
	u.setFileSelected("0", true)
	if u.clear.Disabled() {
		t.Fatal("clear queue is disabled for a populated queue")
	}
	test.Tap(u.clear)
	var dismiss *widget.Button
	walkFileTestObjects(u.win.Canvas().Overlays().Top(), func(object fyne.CanvasObject) {
		if b, ok := object.(*widget.Button); ok && b.Text == "No" {
			dismiss = b
		}
	})
	if dismiss == nil {
		t.Fatal("clear queue confirmation missing")
	}
	test.Tap(dismiss)
	if len(u.engine.Snapshot().Jobs) != 1 {
		t.Fatal("dismissed clear removed a job")
	}
	test.Tap(u.clear)
	var confirm *widget.Button
	walkFileTestObjects(u.win.Canvas().Overlays().Top(), func(object fyne.CanvasObject) {
		if b, ok := object.(*widget.Button); ok && b.Text == "Clear queue" {
			confirm = b
		}
	})
	if confirm == nil {
		t.Fatal("clear queue confirmation action missing")
	}
	test.Tap(confirm)
	deadline := time.Now().Add(3 * time.Second)
	for len(u.engine.Snapshot().Jobs) != 0 || u.engine.Snapshot().Clearing {
		if time.Now().After(deadline) {
			t.Fatal("clear queue did not finish")
		}
		time.Sleep(5 * time.Millisecond)
	}
	u.snapshot = u.engine.Snapshot()
	u.refresh()
	if !u.empty.Visible() || u.details.Visible() || !u.clear.Disabled() || len(u.selectedFiles) != 0 || u.selected != "" {
		t.Fatal("clearing did not restore the empty drop area and selection")
	}
}

func TestJobHeaderPreview(t *testing.T) {
	dir := os.Getenv("EASYDVD_UI_PREVIEW")
	if dir == "" {
		t.Skip("visual preview not requested")
	}
	u := fileListFixture(t)
	for _, appearance := range []string{"Light", "Dark"} {
		u.applyTheme(appearance)
		for _, expanded := range []bool{false, true} {
			u.setHeaderExpanded(expanded)
			u.refresh()
			mode := "collapsed"
			if expanded {
				mode = "expanded"
			}
			f, err := os.Create(filepath.Join(dir, "job-header-"+appearance+"-"+mode+".png"))
			if err != nil {
				t.Fatal(err)
			}
			if err := png.Encode(f, u.win.Canvas().Capture()); err != nil {
				t.Fatal(err)
			}
			f.Close()
		}
	}
}
