package ui

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"easydvd/internal/engine"
	"easydvd/internal/media"
	"easydvd/internal/model"
	"easydvd/internal/store"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

func fileListFixture(t *testing.T) *window {
	t.Helper()
	a := test.NewApp()
	t.Cleanup(a.Quit)
	st, err := store.Open(filepath.Join(t.TempDir(), "queue.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	s := model.DefaultSettings(t.TempDir())
	j := model.Job{ID: model.ID(), Title: "Family collection", Folder: `C:\Videos\Family`, State: model.Ready, Stage: "Ready", ScanComplete: true, EstimatedDiscs: 1, Settings: s}
	for i := 0; i < 18; i++ {
		name := fmt.Sprintf("Holiday-%02d.mp4", i+1)
		j.Videos = append(j.Videos, model.Video{ID: fmt.Sprint(i), Path: filepath.Join(j.Folder, name), Relative: name, Label: strings.TrimSuffix(name, ".mp4"), Duration: 90 + float64(i*10)})
	}
	j.Videos[1].Label = "Summer in Vienna"
	j.Videos[2].Excluded = true
	j.Videos[3].Error = "The video could not be inspected. The file is incomplete or damaged. Try a fresh copy of the source file, or exclude this file from the DVD set."
	j.Videos[3].Duration = 0
	if err := st.Save([]model.Job{j}); err != nil {
		t.Fatal(err)
	}
	e, err := engine.New(st, s, media.Tools{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(e.Close)
	w := a.NewWindow("File list test")
	t.Cleanup(w.Close)
	u := &window{app: a, win: w, engine: e, snapshot: e.Snapshot(), mediaReady: true}
	u.applyTheme("Light")
	w.SetContent(u.build())
	w.Resize(fyne.NewSize(1160, 780))
	u.refresh()
	w.Show()
	return u
}

func waitForFiles(t *testing.T, u *window, matches func([]model.Video) bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		snapshot := u.engine.Snapshot()
		if matches(snapshot.Jobs[0].Videos) {
			u.snapshot = snapshot
			u.refresh()
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("file operation did not finish")
}

func TestCompactHeaderAndFileRows(t *testing.T) {
	u := fileListFixture(t)
	if u.start.Size().Height != u.start.MinSize().Height || u.pause.Size().Height != u.pause.MinSize().Height {
		t.Fatalf("header buttons stretched: start=%v minimum=%v", u.start.Size(), u.start.MinSize())
	}
	r := newFileRow(u)
	r.update(u.selectedJob(), 0)
	if r.MinSize().Height > 40 {
		t.Fatalf("file row is taller than one line: %v", r.MinSize())
	}
	even := r.background.FillColor
	r.update(u.selectedJob(), 1)
	if r.background.FillColor == even {
		t.Fatal("rows do not alternate shading")
	}
	if !r.title.TextStyle.Italic || r.state.Text != "Edited" || r.revert.Disabled() {
		t.Fatal("edited title is not indicated or cannot be reverted")
	}
	r.update(u.selectedJob(), 2)
	if !r.title.TextStyle.Strikethrough || r.state.Text != "Excluded" || r.toggle.hint != "Include file in DVD" {
		t.Fatal("excluded file is not indicated")
	}
	r.update(u.selectedJob(), 3)
	if !r.warning.Visible() {
		t.Fatal("file error has no warning control")
	}
	test.Tap(r.warning)
	var found bool
	walkFileTestObjects(u.win.Canvas().Overlays().Top(), func(object fyne.CanvasObject) {
		if label, ok := object.(*widget.Label); ok && strings.Contains(label.Text, u.selectedJob().Videos[3].Error) {
			found = true
		}
	})
	if !found {
		t.Fatal("warning click did not show the complete error")
	}
	u.win.Canvas().Overlays().Remove(u.win.Canvas().Overlays().Top())
	// List widgets recycle rows; no state or callbacks may leak to the next file.
	r.update(u.selectedJob(), 0)
	if r.warning.Visible() || r.title.TextStyle.Italic || r.title.TextStyle.Strikethrough || r.state.Text != "" || !r.revert.Disabled() {
		t.Fatal("recycled healthy row retained old state")
	}
	test.Tap(r.selectFile)
	if !u.selectedFiles["0"] || u.selectedFiles["3"] {
		t.Fatal("recycled checkbox selected the wrong file")
	}
}

func TestFileSelectionBulkAndPerRowActions(t *testing.T) {
	u := fileListFixture(t)
	for _, index := range []int{0, 1} {
		r := newFileRow(u)
		r.update(u.selectedJob(), index)
		test.Tap(r.selectFile)
	}
	if u.selectionCount.Text != "2 selected" || u.excludeSelected.Disabled() {
		t.Fatal("multi-selection did not enable bulk exclusion")
	}
	test.Tap(u.excludeSelected)
	waitForFiles(t, u, func(v []model.Video) bool { return v[0].Excluded && v[1].Excluded })
	if u.selectedJob().Videos[4].Excluded || !u.excludeSelected.Disabled() || u.includeSelected.Disabled() {
		t.Fatal("bulk exclusion affected another file or left incorrect controls")
	}
	test.Tap(u.includeSelected)
	waitForFiles(t, u, func(v []model.Video) bool { return !v[0].Excluded && !v[1].Excluded })
	if !u.selectedJob().Videos[2].Excluded {
		t.Fatal("bulk inclusion changed an unselected excluded file")
	}
	r := newFileRow(u)
	r.update(u.selectedJob(), 1)
	test.Tap(r.revert)
	waitForFiles(t, u, func(v []model.Video) bool { return v[1].Label == "Holiday-02" })
	r.update(u.selectedJob(), 1)
	if !r.revert.Disabled() || r.state.Text != "" {
		t.Fatal("restoring the original title retained the edited indicator")
	}
	test.Tap(r.toggle)
	waitForFiles(t, u, func(v []model.Video) bool { return v[1].Excluded })
	if u.selectedJob().Videos[1].Label != "Holiday-02" {
		t.Fatal("exclusion changed the video title")
	}
	test.Tap(u.selectAll)
	if len(u.selectedFiles) != len(u.selectedJob().Videos) {
		t.Fatal("select all missed files outside the visible rows")
	}
	test.Tap(u.selectAll)
	if len(u.selectedFiles) != 0 || !u.excludeSelected.Disabled() {
		t.Fatal("deselect all did not clear the selection")
	}
	u.setFileSelected("0", true)
	u.snapshot.Jobs = append(u.snapshot.Jobs, model.Job{ID: "another", State: model.Ready})
	u.selected = "another"
	u.refreshFileSelection()
	if len(u.selectedFiles) != 0 {
		t.Fatal("file selection leaked into another job")
	}
}

func TestFileTitleDialogAndReadOnlyStates(t *testing.T) {
	u := fileListFixture(t)
	r := newFileRow(u)
	r.update(u.selectedJob(), 0)
	test.Tap(r.edit)
	var entry *widget.Entry
	var save *widget.Button
	walkFileTestObjects(u.win.Canvas().Overlays().Top(), func(object fyne.CanvasObject) {
		switch object := object.(type) {
		case *widget.Entry:
			entry = object
		case *widget.Button:
			if object.Text == "Save" {
				save = object
			}
		}
	})
	if entry == nil || save == nil {
		t.Fatal("title editor did not open")
	}
	entry.SetText("   ")
	if !save.Disabled() {
		t.Fatal("blank video title can be saved")
	}
	entry.SetText("  A new title  ")
	test.Tap(save)
	waitForFiles(t, u, func(v []model.Video) bool { return v[0].Label == "A new title" })
	for _, state := range []model.State{model.Running, model.Scanning, model.Complete} {
		u.snapshot.Jobs[0].State = state
		u.refresh()
		r.update(u.selectedJob(), 0)
		if !r.edit.Disabled() || !r.revert.Disabled() || !r.toggle.Disabled() || !u.excludeSelected.Disabled() {
			t.Fatalf("file modifications enabled in state %s", state)
		}
	}
}

func TestIconHoverDoesNotBlockClicks(t *testing.T) {
	u := fileListFixture(t)
	var row *fileRow
	walkFileTestObjects(u.win.Content(), func(object fyne.CanvasObject) {
		if r, ok := object.(*fileRow); ok && r.id == "0" {
			row = r
		}
	})
	if row == nil {
		t.Fatal("visible row not found")
	}
	row.edit.MouseIn(&desktop.MouseEvent{})
	defer row.edit.MouseOut()
	time.Sleep(650 * time.Millisecond)
	if row.edit.help == nil || u.win.Canvas().Overlays().Top() != nil {
		t.Fatal("hover help was not shown or intercepts canvas input")
	}
	pos := u.app.Driver().AbsolutePositionForObject(row.edit)
	pos = pos.Add(fyne.NewPos(row.edit.Size().Width/2, row.edit.Size().Height/2))
	test.TapCanvas(u.win.Canvas(), pos)
	if u.win.Canvas().Overlays().Top() == nil || row.edit.help != nil || len(u.selectedFiles) != 0 {
		t.Fatal("clicking the edit icon after hovering did not open the editor independently of selection")
	}
}

// Optional visual fixture: EASYDVD_UI_PREVIEW is a directory for rendered QA images.
func TestFileListPreview(t *testing.T) {
	dir := os.Getenv("EASYDVD_UI_PREVIEW")
	if dir == "" {
		t.Skip("visual preview not requested")
	}
	u := fileListFixture(t)
	u.setFileSelected("0", true)
	u.setFileSelected("1", true)
	for _, appearance := range []string{"Light", "Dark"} {
		u.applyTheme(appearance)
		u.refresh()
		f, err := os.Create(filepath.Join(dir, "file-list-"+appearance+".png"))
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, u.win.Canvas().Capture()); err != nil {
			t.Fatal(err)
		}
		f.Close()
		// Also inspect unselected stripes, warnings and many recycled rows together.
		u.win.Resize(fyne.NewSize(1160, 980))
		u.selectAll.SetChecked(false)
		u.refresh()
		f, err = os.Create(filepath.Join(dir, "file-list-"+appearance+"-expanded.png"))
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, u.win.Canvas().Capture()); err != nil {
			t.Fatal(err)
		}
		f.Close()
		u.win.Resize(fyne.NewSize(1160, 780))
	}
}

func walkFileTestObjects(object fyne.CanvasObject, visit func(fyne.CanvasObject)) {
	visit(object)
	if c, ok := object.(*fyne.Container); ok {
		for _, child := range c.Objects {
			walkFileTestObjects(child, visit)
		}
	} else if w, ok := object.(fyne.Widget); ok {
		for _, child := range test.WidgetRenderer(w).Objects() {
			walkFileTestObjects(child, visit)
		}
	}
}
