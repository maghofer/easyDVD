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
	locked := false
	for _, job := range u.engine.Snapshot().Jobs {
		if job.ID == id {
			locked = job.Locked
		}
	}
	standard := widget.NewSelect([]string{"PAL", "NTSC"}, nil)
	standard.SetSelected(s.Standard)
	format := widget.NewSelect([]string{"DVD", "Blu-ray (experimental)"}, nil)
	format.SetSelected("DVD")
	if s.IsBluray() {
		format.SetSelected("Blu-ray (experimental)")
	}
	mediaOptions := []string{"DVD-5", "DVD-9"}
	if s.IsBluray() {
		mediaOptions = []string{"BD-25", "BD-50"}
		standard.Disable()
	}
	medium := widget.NewSelect(mediaOptions, nil)
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
	form := widget.NewForm(widget.NewFormItem("Disc format", format), widget.NewFormItem("TV standard", standard), widget.NewFormItem("Disc capacity", medium), widget.NewFormItem("Output folder", folder(output)))
	menuText := "Edit menu…"
	if defaults {
		menuText = "Menu defaults…"
	}
	menuButton := widget.NewButtonWithIcon(menuText, theme.SettingsIcon(), func() { u.menuSettings(id) })
	form.Append("Menu", menuButton)
	workerOptions := []string{"Auto"}
	for n := 1; n <= model.MaxEncoders; n++ {
		workerOptions = append(workerOptions, strconv.Itoa(n))
	}
	workers := widget.NewSelect(workerOptions, nil)
	if s.Encoders == 0 {
		workers.SetSelected("Auto")
	} else {
		workers.SetSelected(strconv.Itoa(s.Encoders))
	}
	profileOptions := []string{s.ProfileLabel(model.ProfileBalanced), s.ProfileLabel(model.ProfileQuality)}
	selectedProfile := s.Profile()
	selectProfile := func(label string) {
		for i, option := range profileOptions {
			if label == option {
				selectedProfile = []string{model.ProfileBalanced, model.ProfileQuality}[i]
			}
		}
	}
	profile := widget.NewSelect(profileOptions, selectProfile)
	profile.SetSelected(s.ProfileLabel(s.Profile()))
	jobProfile := widget.NewRadioGroup(profileOptions, selectProfile)
	jobProfile.Horizontal, jobProfile.Required = true, true
	jobProfile.SetSelected(s.ProfileLabel(s.Profile()))
	heading := widget.NewLabel("")
	var d dialog.Dialog
	dialogSize := func(bluray bool) fyne.Size {
		if defaults {
			return fyne.NewSize(740, 640)
		}
		if bluray {
			return fyne.NewSize(620, 520)
		}
		return fyne.NewSize(620, 460)
	}
	scopeNote := ""
	formatHeading := func() {
		text := scopeNote
		if format.Selected == "Blu-ray (experimental)" {
			text += "\nExperimental Blu-ray uses each source's video format. Standalone-player compatibility is unverified."
		}
		heading.SetText(text)
	}
	format.OnChanged = func(value string) {
		chosen := s
		chosen.DiscFormat = "DVD"
		options := []string{"DVD-5", "DVD-9"}
		standard.Enable()
		if value == "Blu-ray (experimental)" {
			chosen.DiscFormat = "Blu-ray"
			options = []string{"BD-25", "BD-50"}
			standard.Disable()
		}
		medium.SetOptions(options)
		if medium.Selected != options[0] && medium.Selected != options[1] {
			medium.SetSelected(options[0])
		}
		profileOptions = []string{chosen.ProfileLabel(model.ProfileBalanced), chosen.ProfileLabel(model.ProfileQuality)}
		savedProfile := selectedProfile
		profile.SetOptions(profileOptions)
		jobProfile.Options = profileOptions
		jobProfile.Refresh()
		profile.SetSelected(chosen.ProfileLabel(savedProfile))
		jobProfile.SetSelected(chosen.ProfileLabel(savedProfile))
		formatHeading()
		if d != nil {
			d.Resize(dialogSize(chosen.IsBluray()))
		}
	}
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
		form.Append("Encoding profile", profile)
		form.Append("Encoding workers", workers)
		form.Append("Local work folder", folder(work))
		form.Append("Appearance", appearance)
	} else {
		form.Append("Encoding profile", jobProfile)
		form.Append("Encoding workers", workers)
		// Playback mode is part of this job's menu options.
		form.Append("Playback", noMenu)
	}
	if locked {
		form = widget.NewForm(widget.NewFormItem("Encoding workers", workers))
		note = "Change workers for the next resume. Completed encodes are reused; encoding quality settings stay locked."
	}
	scopeNote = note
	formatHeading()
	heading.Wrapping = fyne.TextWrapWord
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
		next.Encoders, _ = strconv.Atoi(workers.Selected) // Auto is stored as zero.
		if !locked {
			if format.Selected == "Blu-ray (experimental)" {
				next.DiscFormat = "Blu-ray"
			} else if next.DiscFormat != "" {
				next.DiscFormat = "DVD"
			}
			next.Standard, next.Media = standard.Selected, medium.Selected
			next.OutputDir = strings.TrimSpace(output.Text)
			if defaults {
				next.Packing, next.EncodingProfile = "Strict order", selectedProfile
				next.WorkDir, next.Theme = strings.TrimSpace(work.Text), appearance.Selected
				next.StageLocally, next.KeepWork = stage.Checked, keep.Checked
			} else {
				next.NoMenu = noMenu.Checked
				next.EncodingProfile = selectedProfile
			}
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
		repair := widget.NewButton("Repair dependencies…", u.diagnostics)
		repair.Importance = widget.LowImportance
		options.Add(container.NewHBox(repair, layout.NewSpacer()))
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
	d.Resize(dialogSize(s.IsBluray()))
	d.Show()
}
