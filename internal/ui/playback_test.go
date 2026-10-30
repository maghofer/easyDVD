package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"strings"
	"testing"
	"time"
)

func TestNoMenuCanOverrideMultipleAndSingleVideoDefaults(t *testing.T) {
	u := fileListFixture(t)
	j := u.selectedJob()
	u.settingsDialog(j.ID, j.Settings)
	var noMenu *widget.Check
	var cancel *widget.Button
	var menuButton *widget.Button
	find := func(object fyne.CanvasObject) {
		switch w := object.(type) {
		case *widget.Check:
			if strings.HasPrefix(w.Text, "No menu") {
				noMenu = w
			}
		case *widget.Button:
			if w.Text == "Edit menu…" {
				menuButton = w
			}
			if w.Text == "Cancel" {
				cancel = w
			}
		}
	}
	walkFileTestObjects(u.win.Canvas().Overlays().Top(), find)
	if noMenu == nil || noMenu.Disabled() || menuButton == nil || menuButton.Disabled() {
		t.Fatal("multiple-video job does not offer playback choices")
	}
	noMenu.SetChecked(true)
	if !menuButton.Disabled() {
		t.Fatal("menu editor enabled with no menu selected")
	}
	noMenu.SetChecked(false)
	if menuButton.Disabled() {
		t.Fatal("menu editor did not enable again")
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
	if !menuButton.Disabled() {
		t.Fatal("single-video menu editor enabled while no menu selected")
	}
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

func TestMultiVideoNoMenuChoicePersistsAndDisablesMenuEditor(t *testing.T) {
	u := fileListFixture(t)
	for _, checked := range []bool{true, false} {
		u.snapshot = u.engine.Snapshot()
		u.settingsDialog(u.selected, u.selectedJob().Settings)
		fields, buttons := settingsFields(u)
		noMenu := fields["Playback"].(*widget.Check)
		menuButton := buttons["Edit menu…"]
		if menuButton.Disabled() != noMenu.Checked {
			t.Fatal("persisted menu editor state disagrees with playback mode")
		}
		noMenu.SetChecked(checked)
		if menuButton.Disabled() != checked {
			t.Fatal("menu editor did not follow checkbox")
		}
		test.Tap(buttons["Save settings"])
		deadline := time.Now().Add(3 * time.Second)
		for {
			j := u.engine.Snapshot().Jobs[0]
			if j.Settings.NoMenu == checked && j.NoMenuOverride != nil && u.win.Canvas().Overlays().Top() == nil {
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("explicit playback choice was not saved")
			}
			time.Sleep(5 * time.Millisecond)
		}
	}
}
