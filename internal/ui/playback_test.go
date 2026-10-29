package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"strings"
	"testing"
	"time"
)

func TestNoMenuOnlySelectableForSingleVideoJob(t *testing.T) {
	u := fileListFixture(t)
	j := u.selectedJob()
	u.settingsDialog(j.ID, j.Settings)
	var noMenu *widget.Check
	var cancel *widget.Button
	find := func(object fyne.CanvasObject) {
		switch w := object.(type) {
		case *widget.Check:
			if strings.HasPrefix(w.Text, "No menu") {
				noMenu = w
			}
		case *widget.Button:
			if w.Text == "Cancel" {
				cancel = w
			}
		}
	}
	walkFileTestObjects(u.win.Canvas().Overlays().Top(), find)
	if noMenu == nil || !noMenu.Disabled() {
		t.Fatal("multiple-video job offered no-menu playback")
	}
	test.Tap(cancel)
	var ids []string
	for _, v := range j.Videos[1:] {
		ids = append(ids, v.ID)
	}
	if err := u.engine.SetExcluded(j.ID, ids, true); err != nil {
		t.Fatal(err)
	}
	u.snapshot = u.engine.Snapshot()
	u.refresh()
	j = u.selectedJob()
	u.settingsDialog(j.ID, j.Settings)
	var save *widget.Button
	walkFileTestObjects(u.win.Canvas().Overlays().Top(), func(object fyne.CanvasObject) {
		find(object)
		if w, ok := object.(*widget.Button); ok && w.Text == "Save settings" {
			save = w
		}
	})
	if noMenu == nil || noMenu.Disabled() || save == nil {
		t.Fatal("single-video playback option unavailable")
	}
	noMenu.SetChecked(true)
	test.Tap(save)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if u.engine.Snapshot().Jobs[0].Settings.NoMenu {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("no-menu choice was not saved")
}
