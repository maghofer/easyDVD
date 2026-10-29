package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"testing"
)

func TestMultipleFileDropOffersGroupingAndCanBeCanceled(t *testing.T) {
	u := fileListFixture(t)
	u.chooseFileGrouping([]string{"first.mp4", "second.mp4"})
	var choice *widget.RadioGroup
	var cancel *widget.Button
	walkFileTestObjects(u.win.Canvas().Overlays().Top(), func(object fyne.CanvasObject) {
		switch w := object.(type) {
		case *widget.RadioGroup:
			choice = w
		case *widget.Button:
			if w.Text == "Cancel" {
				cancel = w
			}
		}
	})
	if choice == nil || len(choice.Options) != 2 || cancel == nil {
		t.Fatal("file grouping choice was not shown")
	}
	if choice.Selected != "All files in one DVD set" {
		t.Fatal(choice.Selected)
	}
	choice.SetSelected("A separate DVD set for each file")
	test.Tap(cancel)
	if len(u.engine.Snapshot().Jobs) != 1 {
		t.Fatal("cancel queued new jobs")
	}
}
