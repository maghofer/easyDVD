package ui

import (
	"easydvd"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

func TestUpdateSettingsInitialState(t *testing.T) {
	a := test.NewApp()
	defer a.Quit()
	w := a.NewWindow("Update settings")
	defer w.Close()
	u := &window{app: a, win: w}
	panel := u.updateSettings()
	w.SetContent(panel)
	w.Resize(fyne.NewSize(650, 210))
	w.Show()
	var check, open *widget.Button
	var hasVersion bool
	walkFileTestObjects(panel, func(o fyne.CanvasObject) {
		switch v := o.(type) {
		case *widget.Button:
			if v.Text == "Check for updates" {
				check = v
			}
			if v.Text == "Open release page" {
				open = v
			}
		case *widget.Label:
			if v.Text == "easyDVD "+easydvd.Version() {
				hasVersion = true
			}
		}
	})
	if check == nil || !check.Visible() || check.Disabled() || open == nil || open.Visible() || open.OnTapped != nil || !hasVersion {
		t.Fatal("update settings must show the current version and manual check, with no release action before a result")
	}
	if dir := os.Getenv("EASYDVD_UI_PREVIEW"); dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		f, err := os.Create(filepath.Join(dir, "update-settings.png"))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, w.Canvas().Capture()); err != nil {
			t.Fatal(err)
		}
	}
}
