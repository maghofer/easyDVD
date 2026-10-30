package ui

import (
	"fmt"
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

func (u *window) settingsDialog(id string, s model.Settings) {
	defaults := id == ""
	standard := widget.NewSelect([]string{"PAL", "NTSC"}, nil)
	standard.SetSelected(s.Standard)
	medium := widget.NewSelect([]string{"DVD-5", "DVD-9"}, nil)
	medium.SetSelected(s.Media)
	output := widget.NewEntry()
	output.SetText(s.OutputDir)
	folder := func(entry *widget.Entry) fyne.CanvasObject {
		browse := widget.NewButtonWithIcon("", theme.FolderOpenIcon(), func() {
			u.action(func() error {
				paths, err := platform.PickFolders(false)
				if err == nil && len(paths) > 0 {
					fyne.Do(func() { entry.SetText(paths[0]) })
				}
				return err
			})
		})
		return container.NewBorder(nil, nil, nil, browse, entry)
	}
	form := widget.NewForm(widget.NewFormItem("TV standard", standard), widget.NewFormItem("Disc capacity", medium), widget.NewFormItem("Output folder", folder(output)))
	menuText := "Edit menu…"
	if defaults {
		menuText = "Menu defaults…"
	}
	menuButton := widget.NewButtonWithIcon(menuText, theme.SettingsIcon(), func() { u.menuSettings(id) })
	form.Append("Menu", menuButton)
	packing := widget.NewSelect([]string{"Strict order", "Space efficiency"}, nil)
	packing.SetSelected(s.Packing)
	workers := widget.NewSelect([]string{"1", "2"}, nil)
	workers.SetSelected(strconv.Itoa(s.Encoders))
	profile := widget.NewSelect([]string{model.ProfileBalanced, model.ProfileQuality}, nil)
	profile.SetSelected(s.Profile())
	jobProfile := widget.NewRadioGroup([]string{model.ProfileBalanced, model.ProfileQuality}, nil)
	jobProfile.Horizontal, jobProfile.Required = true, true
	jobProfile.SetSelected(s.Profile())
	work := widget.NewEntry()
	work.SetText(s.WorkDir)
	appearance := widget.NewSelect([]string{"System", "Light", "Dark"}, nil)
	appearance.SetSelected(s.Theme)
	stage := widget.NewCheck("Stage sources locally (recommended for network shares)", nil)
	stage.SetChecked(s.StageLocally)
	keep := widget.NewCheck("Keep intermediate media after success", nil)
	keep.SetChecked(s.KeepWork)
	noMenu := widget.NewCheck("No menu — play all videos automatically in order", nil)
	noMenu.SetChecked(s.NoMenu)
	playbackEdited := false
	noMenu.OnChanged = func(checked bool) { playbackEdited = true; enabled(menuButton, !checked) }
	if !defaults {
		enabled(menuButton, !s.NoMenu)
	}
	title, note, saveText := "Job settings", "Overrides for this job only. Other jobs and saved defaults stay unchanged.", "Save settings"
	if defaults {
		title, note, saveText = "Default settings", "Every new job starts with these settings. Existing jobs keep their own settings.", "Save as defaults"
		form.Append("File allocation", packing)
		form.Append("Encoding profile", profile)
		form.Append("Encoding workers", workers)
		form.Append("Local work folder", folder(work))
		form.Append("Appearance", appearance)
	} else {
		form.Append("Encoding profile", jobProfile)
		// Playback mode is part of this job's menu options.
		form.Append("Playback", noMenu)
	}
	heading := widget.NewLabel(note)
	heading.Wrapping = fyne.TextWrapWord
	var d dialog.Dialog
	save := func() {
		// Menu options can be edited in their own window while this dialog is
		// open. Merge the fields shown here into the latest saved settings.
		snapshot := u.engine.Snapshot()
		next := snapshot.Settings
		if !defaults {
			found := false
			for _, j := range snapshot.Jobs {
				if j.ID == id {
					next, found = j.Settings, true
					break
				}
			}
			if !found {
				dialog.ShowError(fmt.Errorf("job no longer exists"), u.win)
				return
			}
		}
		next.Standard, next.Media = standard.Selected, medium.Selected
		next.OutputDir = strings.TrimSpace(output.Text)
		if defaults {
			next.Packing, next.EncodingProfile = packing.Selected, profile.Selected
			next.Encoders, _ = strconv.Atoi(workers.Selected)
			next.WorkDir, next.Theme = strings.TrimSpace(work.Text), appearance.Selected
			next.StageLocally, next.KeepWork = stage.Checked, keep.Checked
		} else {
			next.NoMenu = noMenu.Checked
			next.EncodingProfile = jobProfile.Selected
		}
		if err := next.Validate(); err != nil {
			dialog.ShowError(err, u.win)
			return
		}
		u.action(func() error {
			var err error
			if defaults || !playbackEdited {
				err = u.engine.SetSettings(id, next)
			} else {
				err = u.engine.SetJobSettings(id, next)
			}
			if err == nil {
				fyne.Do(func() {
					if defaults {
						u.applyTheme(next.Theme)
					}
					d.Hide()
				})
			}
			return err
		})
	}
	buttons := container.NewHBox(layout.NewSpacer(), widget.NewButton("Cancel", func() { d.Hide() }), widget.NewButton(saveText, save))
	options := container.NewVBox(form)
	if defaults {
		options.Add(stage)
		options.Add(keep)
		options.Add(u.updateSettings())
		buttons.Objects = append([]fyne.CanvasObject{widget.NewButtonWithIcon("Open logs folder", theme.FolderOpenIcon(), func() {
			u.action(func() error {
				if err := os.MkdirAll(u.engine.LogsDir(), 0755); err != nil {
					return err
				}
				return platform.OpenFolder(u.engine.LogsDir())
			})
		})}, buttons.Objects...)
	}
	d = dialog.NewCustomWithoutButtons(title, container.NewBorder(heading, buttons, nil, nil, container.NewVScroll(options)), u.win)
	size := fyne.NewSize(620, 460)
	if defaults {
		size = fyne.NewSize(740, 640)
	}
	d.Resize(size)
	d.Show()
}
