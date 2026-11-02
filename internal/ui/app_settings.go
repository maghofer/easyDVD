package ui

import (
	"os"
	"strconv"
	"strings"

	"easydvd/internal/model"
	"easydvd/internal/platform"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func (u *window) appSettingsDialog() {
	s := u.engine.Snapshot().Settings
	workers := widget.NewSelect(workerChoices(), nil)
	workers.SetSelected("Auto")
	if s.Encoders > 0 {
		workers.SetSelected(strconv.Itoa(s.Encoders))
	}
	work := widget.NewEntry()
	work.SetText(s.WorkDir)
	browse := widget.NewButtonWithIcon("", theme.FolderOpenIcon(), func() {
		u.action(func() error {
			paths, err := platform.PickFolders(false)
			if err == nil && len(paths) > 0 {
				fyne.Do(func() { work.SetText(paths[0]) })
			}
			return err
		})
	})
	appearance := widget.NewSelect([]string{"System", "Light", "Dark"}, nil)
	appearance.SetSelected(s.Theme)
	stage := widget.NewCheck("Stage sources locally", nil)
	stage.SetChecked(s.StageLocally)
	keep := widget.NewCheck("Keep intermediate media after success", nil)
	keep.SetChecked(s.KeepWork)
	updates := widget.NewCheck("Check for updates on application start", nil)
	updates.SetChecked(s.AutoCheckUpdates)
	form := widget.NewForm(widget.NewFormItem("Encoding workers", workers), widget.NewFormItem("Temporary work folder", container.NewBorder(nil, nil, nil, browse, work)), widget.NewFormItem("Appearance", appearance))
	note := widget.NewLabel("Workers apply when the next job starts or resumes. Active stages keep their current settings.")
	note.Wrapping = fyne.TextWrapWord
	logs := widget.NewButtonWithIcon("Open logs", theme.FolderOpenIcon(), func() {
		u.action(func() error {
			if err := os.MkdirAll(u.engine.LogsDir(), 0755); err != nil {
				return err
			}
			return platform.OpenFolder(u.engine.LogsDir())
		})
	})
	var d *dialog.CustomDialog
	save := widget.NewButton("Save", func() {
		next := u.engine.Snapshot().Settings
		next.Encoders, _ = strconv.Atoi(workers.Selected)
		next.WorkDir = strings.TrimSpace(work.Text)
		next.Theme = appearance.Selected
		next.StageLocally, next.KeepWork, next.AutoCheckUpdates = stage.Checked, keep.Checked, updates.Checked
		next.AppSettingsVersion = 1
		u.action(func() error {
			err := u.engine.SetSettings("", next)
			if err == nil {
				fyne.Do(func() { u.applyTheme(next.Theme); d.Hide() })
			}
			return err
		})
	})
	body := container.NewVBox(form, note, stage, keep, updates, u.updateSettings(), widget.NewButton("Media tools and dependencies…", u.diagnostics))
	d = dialog.NewCustomWithoutButtons("Settings", container.NewBorder(nil, container.NewHBox(logs, layout.NewSpacer(), widget.NewButton("Cancel", func() { d.Hide() }), save), nil, nil, container.NewVScroll(body)), u.win)
	d.Resize(fyne.NewSize(700, 600))
	d.Show()
}

func workerChoices() []string {
	out := []string{"Auto"}
	for i := 1; i <= model.MaxEncoders; i++ {
		out = append(out, strconv.Itoa(i))
	}
	return out
}
