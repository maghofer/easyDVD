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

func TestJobHeaderHasFixedCompactInformation(t *testing.T) {
	u := fileListFixture(t)
	height := u.jobHeader.Size().Height
	if height > 100 || u.files.Size().Height < 300 {
		t.Fatalf("header is too tall: %v", u.jobHeader.Size())
	}
	if !u.source.Visible() || u.source.Text != u.selectedJob().Folder || !u.settings.Visible() || !u.renameJob.Visible() {
		t.Fatal("compact header hides job information or actions")
	}
	if !u.jobOutput.Visible() || u.jobOutput.Text != "Output: "+u.selectedJob().Settings.OutputDir {
		t.Fatal("output folder is not shown in the compact header")
	}
	titlePos := u.app.Driver().AbsolutePositionForObject(u.title)
	penPos := u.app.Driver().AbsolutePositionForObject(u.renameJob)
	gap := penPos.X - titlePos.X - u.title.Size().Width
	if gap < 0 || gap > 8 {
		t.Fatalf("title-edit icon is not beside the title: gap=%v", gap)
	}
	var openOutput bool
	var outputButton *widget.Button
	walkFileTestObjects(u.jobHeader, func(object fyne.CanvasObject) {
		if b, ok := object.(*widget.Button); ok && b.Text == "Open output" {
			openOutput = true
			outputButton = b
		}
		if b, ok := object.(*widget.Button); ok && b.Text == "Menu" {
			t.Fatal("redundant menu button is still in the header")
		}
		if _, ok := object.(*widget.RadioGroup); ok {
			t.Fatal("encoding controls are still in the header")
		}
		if label, ok := object.(*widget.Label); ok && label.Text == string(u.selectedJob().State) {
			t.Fatal("job status is repeated in the header")
		}
	})
	if !openOutput {
		t.Fatal("output action has no text label")
	}
	settingsPos := u.app.Driver().AbsolutePositionForObject(u.settings)
	outputPos := u.app.Driver().AbsolutePositionForObject(outputButton)
	if settingsPos.Y != outputPos.Y {
		t.Fatal("Job settings and Open output are not on the same row")
	}
	if !u.jobWarning.Visible() || u.jobWarning.Text != "1 file issue" {
		t.Fatal("file issue was hidden")
	}
	u.snapshot.Jobs[0].Folder = "C:/A very long source path containing a family video collection and several nested folders"
	u.snapshot.Jobs[0].Settings.Standard = "NTSC"
	u.snapshot.Jobs[0].Settings.Media = "DVD-9"
	u.snapshot.Jobs[0].Settings.Packing = "Space efficiency"
	u.snapshot.Jobs[0].Error = "A detailed error that must never expand the job header"
	u.snapshot.Jobs[0].OutputPath = filepath.Join(t.TempDir(), "completed-output")
	u.refresh()
	if u.jobOutput.Text != "Output: "+u.snapshot.Jobs[0].OutputPath {
		t.Fatal("completed job does not show its actual output folder")
	}
	if u.jobHeader.Size().Height != height {
		t.Fatal("header resized or lost format information")
	}
	other := u.snapshot.Jobs[0].Clone()
	other.ID = "another-job"
	u.snapshot.Jobs = append(u.snapshot.Jobs, other)
	u.selected = other.ID
	u.refresh()
	if u.jobHeader.Size().Height != height {
		t.Fatal("selecting another job changed header height")
	}
}

func TestToolsUsesOnlyExclamationSymbol(t *testing.T) {
	u := fileListFixture(t)
	var symbol bool
	walkFileTestObjects(u.win.Content(), func(object fyne.CanvasObject) {
		if b, ok := object.(*fileAction); ok && b.hint == "Media tools" {
			if b.Text != "!" || b.Icon != nil {
				t.Fatal("tools control is not symbol-only")
			}
			symbol = true
		}
		if b, ok := object.(*widget.Button); ok && b.Text == "Tools" {
			t.Fatal("old Tools label is still visible")
		}
	})
	if !symbol {
		t.Fatal("media tools control missing")
	}
}
func TestFooterProgressFollowsActivity(t *testing.T) {
	u := fileListFixture(t)
	for _, tc := range []struct {
		state    model.State
		progress float64
		visible  bool
	}{
		{model.Scanning, 0, true},
		{model.Ready, 0, false},
		{model.Running, 0, true},
		{model.Running, .45, true},
		{model.Paused, .45, true},
		{model.Paused, 0, false},
		{model.Blocked, 0, false},
		{model.Canceled, .3, false},
		{model.Complete, 1, false},
	} {
		u.snapshot.Jobs[0].State = tc.state
		u.snapshot.Jobs[0].Progress = tc.progress
		u.refresh()
		if u.footerProgress.Visible() != tc.visible || u.jobHeader.Size().Height > 100 {
			t.Fatalf("incorrect progress visibility for %s at %v", tc.state, tc.progress)
		}
	}
}

func TestClearQueueConfirmationAndEmptyState(t *testing.T) {
	u := fileListFixture(t)
	u.selectFiles("0", 0)
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
		u.refresh()
		f, err := os.Create(filepath.Join(dir, "job-header-"+appearance+"-fixed.png"))
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, u.win.Canvas().Capture()); err != nil {
			t.Fatal(err)
		}
		f.Close()
	}
}
