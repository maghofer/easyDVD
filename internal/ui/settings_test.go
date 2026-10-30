package ui

import (
	"easydvd/internal/model"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

func settingsFields(u *window) (map[string]fyne.CanvasObject, map[string]*widget.Button) {
	fields := make(map[string]fyne.CanvasObject)
	buttons := make(map[string]*widget.Button)
	walkFileTestObjects(u.win.Canvas().Overlays().Top(), func(object fyne.CanvasObject) {
		if f, ok := object.(*widget.Form); ok {
			for _, item := range f.Items {
				fields[item.Text] = item.Widget
			}
		}
		if b, ok := object.(*widget.Button); ok {
			buttons[b.Text] = b
		}
	})
	return fields, buttons
}

func TestDefaultSettingsAndJobOverrideScope(t *testing.T) {
	u := fileListFixture(t)
	original := u.snapshot.Jobs[0].Settings
	u.settingsDialog("", u.snapshot.Settings)
	fields, buttons := settingsFields(u)
	if fields["Titles per page"] != nil || fields["Menu layout"] != nil || buttons["Save as defaults"] == nil {
		t.Fatal("default settings duplicate menu controls or have no explicit default save")
	}
	if fields["Encoding workers"] == nil || fields["Local work folder"] == nil {
		t.Fatal("default options were lost")
	}
	fields["TV standard"].(*widget.Select).SetSelected("NTSC")
	// Save a menu default while the main dialog is open; it must not be overwritten.
	style := u.snapshot.Settings.Menu
	style.Aspect = "16:9"
	if err := u.engine.SetMenu("", style, "Text", 7); err != nil {
		t.Fatal(err)
	}
	test.Tap(buttons["Save as defaults"])
	deadline := time.Now().Add(3 * time.Second)
	for u.engine.Snapshot().Settings.Standard != "NTSC" || u.win.Canvas().Overlays().Top() != nil {
		if time.Now().After(deadline) {
			t.Fatal("defaults were not saved")
		}
		time.Sleep(5 * time.Millisecond)
	}
	snapshot := u.engine.Snapshot()
	if snapshot.Settings.TitlesPerPage != 7 || snapshot.Settings.Menu.Aspect != "16:9" || snapshot.Jobs[0].Settings != original {
		t.Fatal("saving defaults changed an existing job or lost menu defaults")
	}
	u.snapshot = snapshot
	u.refresh()
	u.settingsDialog(u.selected, original)
	fields, buttons = settingsFields(u)
	for _, name := range []string{"Encoding workers", "File allocation", "Local work folder", "Appearance", "Titles per page"} {
		if fields[name] != nil {
			t.Fatalf("job overrides expose default-only option %s", name)
		}
	}
	for _, name := range []string{"TV standard", "Disc capacity", "Output folder", "Menu", "Encoding profile"} {
		if fields[name] == nil {
			t.Fatalf("missing job override %s", name)
		}
	}
	fields["Disc capacity"].(*widget.Select).SetSelected("DVD-9")
	profile, ok := fields["Encoding profile"].(*widget.RadioGroup)
	if !ok || profile.Selected != model.ProfileBalanced {
		t.Fatal("job settings have no encoding radio buttons")
	}
	profile.SetSelected(model.ProfileQuality)
	jobMenu := original.Menu
	jobMenu.Font = "Monospace"
	if err := u.engine.SetMenu(u.selected, jobMenu, "Text", 6); err != nil {
		t.Fatal(err)
	}
	test.Tap(buttons["Save settings"])
	deadline = time.Now().Add(3 * time.Second)
	for u.engine.Snapshot().Jobs[0].Settings.Media != "DVD-9" {
		if time.Now().After(deadline) {
			t.Fatal("job override was not saved")
		}
		time.Sleep(5 * time.Millisecond)
	}
	snapshot = u.engine.Snapshot()
	job := snapshot.Jobs[0].Settings
	if job.Profile() != model.ProfileQuality || snapshot.Settings.Profile() != model.ProfileBalanced {
		t.Fatal("job profile did not persist independently of defaults")
	}
	if snapshot.Settings.Media != "DVD-5" || snapshot.Settings.Standard != "NTSC" || job.Standard != "PAL" || job.TitlesPerPage != 6 || job.Menu.Font != "Monospace" || job.Encoders != original.Encoders || job.WorkDir != original.WorkDir {
		t.Fatal("job override changed defaults, hidden options or menu changes")
	}
}

func TestSettingsPreview(t *testing.T) {
	dir := os.Getenv("EASYDVD_UI_PREVIEW")
	if dir == "" {
		t.Skip("visual preview not requested")
	}
	u := fileListFixture(t)
	u.applyTheme("Dark")
	for _, id := range []string{"", u.selected} {
		u.settingsDialog(id, u.snapshot.Settings)
		mode := "job"
		if id == "" {
			mode = "defaults"
		}
		f, err := os.Create(filepath.Join(dir, "settings-"+mode+".png"))
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, u.win.Canvas().Capture()); err != nil {
			t.Fatal(err)
		}
		f.Close()
		_, buttons := settingsFields(u)
		test.Tap(buttons["Cancel"])
	}
}
