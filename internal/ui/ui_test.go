package ui

import (
	"easydvd/internal/engine"
	"easydvd/internal/model"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"testing"
)

func TestJobTitleGetsAvailableWidth(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	w := a.NewWindow("Layout test")
	defer w.Close()
	settings := model.DefaultSettings(t.TempDir())
	j := model.Job{ID: "fixture", Title: "Film collection 5694", State: model.Ready, ScanComplete: true, Settings: settings, Videos: []model.Video{{ID: "one", Label: "5694-001", Duration: 30}}}
	u := &window{app: a, win: w, snapshot: engine.Snapshot{Jobs: []model.Job{j}, Settings: settings, Busy: map[string]bool{}}}
	w.SetContent(u.build())
	w.Resize(fyne.NewSize(1160, 780))
	u.refresh()
	w.Show()
	if u.title.Text != j.Title || u.title.Size().Width < 200 {
		t.Fatalf("title was squeezed out: text=%q size=%v", u.title.Text, u.title.Size())
	}
	if !u.dropBar.Visible() || u.empty.Visible() {
		t.Fatal("populated job view is not visible")
	}
}
func TestEmptyStateUsesCentralDropArea(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	w := a.NewWindow("Empty test")
	defer w.Close()
	u := &window{app: a, win: w, snapshot: engine.Snapshot{Settings: model.DefaultSettings(t.TempDir())}}
	w.SetContent(u.build())
	w.Resize(fyne.NewSize(1160, 780))
	u.refresh()
	if !u.empty.Visible() || u.dropBar.Visible() || u.details.Visible() {
		t.Fatal("empty drop state is not centered in the main content area")
	}
}
