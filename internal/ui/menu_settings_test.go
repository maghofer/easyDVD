package ui

import (
	"easydvd/internal/model"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMenuSettingsApplyPreviewAndPresets(t *testing.T) {
	u := fileListFixture(t)
	if u.preview.Text != "Menu" {
		t.Fatal("job button must open menu settings")
	}
	w := u.menuSettings(u.selected)
	if w == nil {
		t.Fatal("missing menu settings")
	}
	defer w.Close()
	var aspect, fontChoice, preset *widget.Select
	var colorField *widget.Entry
	var apply, savePreset *widget.Button
	var status *widget.Label
	walkFileTestObjects(w.Content(), func(o fyne.CanvasObject) {
		switch v := o.(type) {
		case *widget.Select:
			if len(v.Options) == 2 && v.Options[0] == "4:3" {
				aspect = v
			}
			if len(v.Options) == 3 && v.Options[0] == "Sans" {
				fontChoice = v
			}
			if len(v.Options) >= 2 && v.Options[0] == "Current settings" {
				preset = v
			}
		case *widget.Entry:
			if v.Text == "#ECF0F6" {
				colorField = v
			}
		case *widget.Button:
			if v.Text == "Apply to job" {
				apply = v
			}
			if v.Text == "Save preset…" {
				savePreset = v
			}
		case *widget.Label:
			if strings.Contains(v.Text, "Preparing preview") || strings.Contains(v.Text, "Rendering preview") {
				status = v
			}
		}
	})
	if aspect == nil || fontChoice == nil || colorField == nil || apply == nil || savePreset == nil || preset == nil || status == nil {
		t.Fatal("missing required menu controls")
	}
	aspect.SetSelected("16:9")
	fontChoice.SetSelected("Monospace")
	colorField.SetText("#FFEFD5")
	deadline := time.Now().Add(3 * time.Second)
	for !strings.HasPrefix(status.Text, "Preview rendered") {
		if time.Now().After(deadline) {
			t.Fatal("preview did not render", status.Text)
		}
		time.Sleep(10 * time.Millisecond)
	}
	test.Tap(apply)
	deadline = time.Now().Add(3 * time.Second)
	for u.engine.Snapshot().Jobs[0].Settings.Menu.Aspect != "16:9" {
		if time.Now().After(deadline) {
			t.Fatal("menu settings did not apply")
		}
		time.Sleep(5 * time.Millisecond)
	}
	saved := u.engine.Snapshot()
	if saved.Settings.Menu.Aspect != "4:3" || saved.Jobs[0].Settings.Menu.Font != "Monospace" || saved.Jobs[0].Settings.Menu.FontColor != "#FFEFD5" {
		t.Fatal("wrong scope or typography saved")
	}
	test.Tap(savePreset)
	var name *widget.Entry
	var confirm *widget.Button
	walkFileTestObjects(w.Canvas().Overlays().Top(), func(o fyne.CanvasObject) {
		if v, ok := o.(*widget.Entry); ok {
			name = v
		}
		if v, ok := o.(*widget.Button); ok && v.Text == "Save" {
			confirm = v
		}
	})
	if name == nil || confirm == nil {
		t.Fatal("save preset dialog missing")
	}
	name.SetText("Widescreen archive")
	test.Tap(confirm)
	deadline = time.Now().Add(3 * time.Second)
	for {
		p, e := u.engine.MenuPresets()
		if e != nil {
			t.Fatal(e)
		}
		if len(p) == 1 {
			if p[0].Style.Aspect != "16:9" {
				t.Fatal(p)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("preset did not save")
		}
		time.Sleep(5 * time.Millisecond)
	}
	preset.SetSelected("Default")
	if aspect.Selected != "4:3" {
		t.Fatal("default preset did not load")
	}
	deadline = time.Now().Add(3 * time.Second)
	for {
		found := false
		for _, n := range preset.Options {
			if n == "Widescreen archive" {
				found = true
			}
		}
		if found {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("saved preset not listed")
		}
		time.Sleep(5 * time.Millisecond)
	}
	preset.SetSelected("Widescreen archive")
	if aspect.Selected != "16:9" || fontChoice.Selected != "Monospace" {
		t.Fatal("saved preset not loaded")
	}
	time.Sleep(250 * time.Millisecond)
	if dir := os.Getenv("EASYDVD_UI_PREVIEW"); dir != "" {
		f, e := os.Create(filepath.Join(dir, "menu-settings-wide.png"))
		if e != nil {
			t.Fatal(e)
		}
		if e = png.Encode(f, w.Canvas().Capture()); e != nil {
			t.Fatal(e)
		}
		f.Close()
	}
}

func TestFinishedMenuCanPreviewAndSavePresets(t *testing.T) {
	u := fileListFixture(t)
	u.snapshot.Jobs[0].Locked = true
	u.snapshot.Jobs[0].State = model.Complete
	w := u.menuSettings(u.selected)
	defer w.Close()
	found := false
	walkFileTestObjects(w.Content(), func(o fyne.CanvasObject) {
		if v, ok := o.(*widget.Button); ok && v.Text == "Apply to job" {
			found = true
			if !v.Disabled() {
				t.Fatal("finished ISO settings are editable")
			}
		}
	})
	if !found {
		t.Fatal("missing job apply control")
	}
}
