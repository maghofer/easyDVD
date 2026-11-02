package ui

import (
	"context"
	"errors"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"easydvd/internal/model"
	"easydvd/internal/release"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

func TestInlineBatchHeaderEditsOneFieldForEverySelectedJob(t *testing.T) {
	u := jobFixture(t)
	ids := []string{u.snapshot.Jobs[0].ID, u.snapshot.Jobs[1].ID}
	s := u.snapshot.Jobs[1].Settings
	s.Standard = "NTSC"
	s.EncodingProfile = model.ProfileQuality
	if err := u.engine.SetSettings(ids[1], s); err != nil {
		t.Fatal(err)
	}
	u.snapshot = u.engine.Snapshot()
	u.selectJobs(ids[0], 0)
	u.selectJobs(ids[1], fyne.KeyModifierControl)
	if u.headerScope.Text != "Editing 2 jobs" || !u.headerScope.Visible() || u.headerEncoding.Selected != "" || u.headerStandard.Selected != "" {
		t.Fatal("mixed batch scope missing")
	}
	test.Tap(u.headerMedia.buttons[1])
	deadline := time.Now().Add(3 * time.Second)
	for {
		snap := u.engine.Snapshot()
		if snap.Jobs[0].Settings.Media == "DVD-9" && snap.Jobs[1].Settings.Media == "DVD-9" {
			u.snapshot = snap
			u.refresh()
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("batch change did not save")
		}
		time.Sleep(5 * time.Millisecond)
	}
	if u.snapshot.Jobs[0].Settings.Standard != "PAL" || u.snapshot.Jobs[1].Settings.Standard != "NTSC" || u.snapshot.Jobs[1].Settings.Profile() != model.ProfileQuality {
		t.Fatal("batch capacity edit overwrote mixed values")
	}
	if dir := os.Getenv("EASYDVD_UI_PREVIEW"); dir != "" {
		f, err := os.Create(filepath.Join(dir, "job-header-batch.png"))
		if err != nil {
			t.Fatal(err)
		}
		err = png.Encode(f, u.win.Canvas().Capture())
		f.Close()
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestHeaderProfileButtonsAndMenuPresetsApplyToSelectionAndRespectPlayback(t *testing.T) {
	u := jobFixture(t)
	ids := []string{u.snapshot.Jobs[0].ID, u.snapshot.Jobs[1].ID}
	preset := model.EncodingPreset{Name: "Archive DVD", DiscFormat: "DVD", Standard: "NTSC", Media: "DVD-9", EncodingProfile: model.ProfileQuality}
	style := u.snapshot.Settings.Menu
	style.Aspect = "16:9"
	menu := model.MenuPreset{Name: "Wide menu", Style: style, Layout: "Text", TitlesPerPage: 6}
	if err := u.engine.SaveEncodingPreset(preset); err != nil {
		t.Fatal(err)
	}
	if err := u.engine.SaveMenuPreset(menu); err != nil {
		t.Fatal(err)
	}
	u.loadHeaderPresets()
	u.selectJobs(ids[0], 0)
	u.selectJobs(ids[1], fyne.KeyModifierControl)
	test.Tap(u.headerEncoding.buttons[1])
	wait := func(check func([]model.Job) bool) {
		t.Helper()
		deadline := time.Now().Add(3 * time.Second)
		for time.Now().Before(deadline) {
			snap := u.engine.Snapshot()
			if check(snap.Jobs) {
				u.snapshot = snap
				u.refresh()
				return
			}
			time.Sleep(5 * time.Millisecond)
		}
		t.Fatal("header setting did not apply")
	}
	wait(func(jobs []model.Job) bool {
		return jobs[0].Settings.Profile() == model.ProfileQuality && jobs[1].Settings.Profile() == model.ProfileQuality
	})
	if u.headerEncoding.Selected != model.ProfileQuality || u.headerEncoding.buttons[1].Importance != widget.HighImportance {
		t.Fatal("quality button is not highlighted")
	}
	if u.snapshot.Jobs[0].Settings.Media != "DVD-5" || u.snapshot.Jobs[1].Settings.Standard != "PAL" {
		t.Fatal("profile button applied an encoding preset")
	}
	if saved, err := u.engine.EncodingPresets(); err != nil || len(saved) != 1 || saved[0] != preset {
		t.Fatal("disconnected encoding preset was lost", err)
	}
	u.headerMenuPreset.SetSelected("Preset: Wide menu")
	wait(func(jobs []model.Job) bool {
		return jobs[0].Settings.Menu == style && jobs[1].Settings.TitlesPerPage == 6
	})
	if u.headerMenuPreset.Selected != "Preset: Wide menu" {
		t.Fatal("applied menu preset is not selected")
	}
	test.Tap(u.headerPlayback.buttons[1])
	wait(func(jobs []model.Job) bool { return jobs[0].Settings.NoMenu && jobs[1].Settings.NoMenu })
	if !u.headerMenuPreset.Disabled() || u.headerPlayback.buttons[1].Importance != widget.HighImportance {
		t.Fatal("autoplay did not disable menu presets or highlight the active button")
	}
	test.Tap(u.headerPlayback.buttons[0])
	wait(func(jobs []model.Job) bool { return !jobs[0].Settings.NoMenu && !jobs[1].Settings.NoMenu })
	if u.headerMenuPreset.Disabled() {
		t.Fatal("menu presets did not re-enable")
	}
	test.Tap(u.blurayButton)
	wait(func(jobs []model.Job) bool { return jobs[0].Settings.IsBluray() && jobs[1].Settings.IsBluray() })
	if u.headerMedia.Options[0] != "BD-25" || u.headerStandard.Visible() || u.blurayButton.Importance != widget.HighImportance {
		t.Fatal("Blu-ray choices did not update")
	}
	test.Tap(u.headerEncoding.buttons[0])
	wait(func(jobs []model.Job) bool {
		return jobs[0].Settings.Profile() == model.ProfileBalanced && jobs[1].Settings.Profile() == model.ProfileBalanced
	})
	if u.snapshot.Settings.Media != "DVD-5" || u.snapshot.Jobs[2].Settings.Media != "DVD-5" {
		t.Fatal("preset selection changed defaults or unselected jobs")
	}
	u.snapshot.Jobs[0].Locked = true
	u.refresh()
	if !u.editOutput.Disabled() || !u.headerEncoding.Disabled() || !u.headerMedia.Disabled() {
		t.Fatal("locked selection exposes editable header settings")
	}
}

func TestFileAndQueueDragReorderAtScrolledPositions(t *testing.T) {
	u := jobFixture(t)
	u.selectFiles("0", 0)
	u.selectFiles("1", fyne.KeyModifierControl)
	r := newFileRow(u)
	r.update(u.selectedJob(), 0)
	row := r.MinSize().Height + 8
	origin := u.app.Driver().AbsolutePositionForObject(u.files)
	r.Dragged(&fyne.DragEvent{PointEvent: fyne.PointEvent{AbsolutePosition: origin.Add(fyne.NewPos(50, 4*row))}})
	r.DragEnd()
	waitForFiles(t, u, func(v []model.Video) bool {
		return v[0].ID == "2" && v[1].ID == "3" && v[2].ID == "0" && v[3].ID == "1"
	})
	jobs := u.snapshot.Jobs
	u.selectJobs(jobs[0].ID, 0)
	u.selectJobs(jobs[1].ID, fyne.KeyModifierControl)
	tile := newJobTile(u)
	tile.update(jobs[0])
	origin = u.app.Driver().AbsolutePositionForObject(u.queue)
	tile.Dragged(&fyne.DragEvent{PointEvent: fyne.PointEvent{AbsolutePosition: origin.Add(fyne.NewPos(40, 4*54))}})
	tile.DragEnd()
	deadline := time.Now().Add(3 * time.Second)
	for {
		s := u.engine.Snapshot()
		if s.Jobs[0].ID == jobs[2].ID && s.Jobs[2].ID == jobs[0].ID {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("queue drag did not move selection")
		}
		time.Sleep(5 * time.Millisecond)
	}
	if u.win.Canvas().Overlays().Top() != nil {
		t.Fatal("drag indicator still intercepts input")
	}
}

func TestAppSettingsExposeOnlyApplicationOptions(t *testing.T) {
	u := fileListFixture(t)
	u.appSettingsDialog()
	fields, buttons := settingsFields(u)
	for _, name := range []string{"Disc format", "Encoding profile", "TV standard", "File allocation", "Output folder"} {
		if fields[name] != nil {
			t.Fatal("job option in app settings", name)
		}
	}
	if fields["Encoding workers"].(*widget.Select).Selected != "Auto" || fields["Temporary work folder"] == nil || buttons["Open logs"] == nil {
		t.Fatal("application controls missing")
	}
	test.Tap(buttons["Cancel"])
}

func TestStartupUpdateChecksAreQuietExceptForNewReleases(t *testing.T) {
	for _, kind := range []string{"current", "failed", "available", "disabled"} {
		t.Run(kind, func(t *testing.T) {
			u := fileListFixture(t)
			u.closed = make(chan struct{})
			defer close(u.closed)
			called := make(chan struct{})
			u.snapshot.Settings.AutoCheckUpdates = kind != "disabled"
			u.updateCheck = func(context.Context) (release.Result, error) {
				defer close(called)
				if kind == "failed" {
					return release.Result{}, errors.New("offline")
				}
				return release.Result{Available: kind == "available", Version: "9.0.0", URL: "https://github.com/example/easydvd/releases/tag/v9.0.0"}, nil
			}
			u.checkUpdatesOnLaunch()
			if kind == "disabled" {
				select {
				case <-called:
					t.Fatal("disabled startup check ran")
				case <-time.After(30 * time.Millisecond):
				}
				return
			}
			<-called
			deadline := time.Now().Add(time.Second)
			if kind == "available" {
				for !u.updateNotice.Visible() {
					if time.Now().After(deadline) {
						t.Fatal("new release not shown")
					}
					time.Sleep(time.Millisecond)
				}
			} else {
				time.Sleep(20 * time.Millisecond)
				if u.updateNotice.Visible() || u.win.Canvas().Overlays().Top() != nil {
					t.Fatal("startup check interrupted user")
				}
			}
		})
	}
}

func TestCompletedJobContextMenuAndElapsedTime(t *testing.T) {
	u := jobFixture(t)
	j := u.snapshot.Jobs[3]
	j.ElapsedSeconds = 125
	tile := newJobTile(u)
	tile.update(j)
	if !strings.Contains(tile.state.Text, "00:02:05") {
		t.Fatal("elapsed time missing", tile.state.Text)
	}
	tile.TappedSecondary(&fyne.PointEvent{AbsolutePosition: fyne.NewPos(100, 220)})
	menu := tile.contextMenu()
	if u.win.Canvas().Overlays().Top() == nil || len(menu.Items) != 1 || menu.Items[0].Label != "Clear render status" || menu.Items[0].Disabled {
		t.Fatal("completed job has no reset action")
	}
	menu.Items[0].Action()
	deadline := time.Now().Add(3 * time.Second)
	for u.engine.Snapshot().Jobs[3].RenderRevision != 1 {
		if time.Now().After(deadline) {
			t.Fatal("context reset did not run")
		}
		time.Sleep(5 * time.Millisecond)
	}
}
