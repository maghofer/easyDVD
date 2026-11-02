package ui

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

func TestBluRaySettingsSwitchFormatAndPreserveProfile(t *testing.T) {
	u := fileListFixture(t)
	u.settingsDialog(u.selected, u.snapshot.Jobs[0].Settings)
	fields, buttons := settingsFields(u)
	format := fields["Disc format"].(*widget.Select)
	profile := fields["Encoding profile"].(*widget.RadioGroup)
	profile.SetSelected("Quality (hcenc)")
	format.SetSelected("Blu-ray (experimental)")
	if !fields["TV standard"].(*widget.Select).Disabled() || fields["Disc capacity"].(*widget.Select).Selected != "BD-25" || profile.Selected != "Quality (x264)" {
		t.Fatal("Blu-ray controls did not switch")
	}
	fields["Disc capacity"].(*widget.Select).SetSelected("BD-50")
	if root := os.Getenv("EASYDVD_BLURAY_UI_PREVIEW"); root != "" {
		f, err := os.Create(filepath.Join(root, "bluray-settings.png"))
		if err != nil {
			t.Fatal(err)
		}
		err = png.Encode(f, u.win.Canvas().Capture())
		f.Close()
		if err != nil {
			t.Fatal(err)
		}
	}
	test.Tap(buttons["Save settings"])
	end := time.Now().Add(3 * time.Second)
	for !u.engine.Snapshot().Jobs[0].Settings.IsBluray() {
		if time.Now().After(end) {
			t.Fatal("settings not saved")
		}
		time.Sleep(5 * time.Millisecond)
	}
	s := u.engine.Snapshot().Jobs[0].Settings
	if s.Media != "BD-50" || s.Profile() != "Quality" || u.engine.Snapshot().Settings.IsBluray() {
		t.Fatal(s)
	}
}
