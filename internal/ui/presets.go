package ui

import (
	"fmt"
	"strings"

	"easydvd/internal/model"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

func (u *window) encodingPresetsDialog() {
	ids := u.headerJobIDs()
	presets, err := u.engine.EncodingPresets()
	if err != nil {
		dialog.ShowError(err, u.win)
		return
	}
	s := u.snapshot.Settings
	if j := u.selectedJob(); j != nil {
		s = j.Settings
	}
	name := widget.NewEntry()
	name.SetPlaceHolder("Preset name")
	format := widget.NewSelect([]string{"DVD", "Blu-ray"}, nil)
	format.SetSelected(s.Format())
	standard := widget.NewSelect([]string{"PAL", "NTSC"}, nil)
	standard.SetSelected(s.Standard)
	medium := widget.NewSelect(nil, nil)
	profileID := s.Profile()
	profile := widget.NewSelect([]string{s.ProfileLabel(model.ProfileBalanced), s.ProfileLabel(model.ProfileQuality)}, nil)
	profile.SetSelected(s.ProfileLabel(profileID))
	profile.OnChanged = func(value string) {
		for _, p := range []string{model.ProfileBalanced, model.ProfileQuality} {
			chosen := s
			chosen.DiscFormat = format.Selected
			if value == chosen.ProfileLabel(p) {
				profileID = p
			}
		}
	}
	setFormat := func(f string) {
		options := []string{"DVD-5", "DVD-9"}
		if f == "Blu-ray" {
			options = []string{"BD-25", "BD-50"}
		}
		medium.SetOptions(options)
		if medium.Selected != options[0] && medium.Selected != options[1] {
			medium.SetSelected(options[0])
		}
		enabled(standard, f == "DVD")
		chosen := s
		chosen.DiscFormat = f
		profile.SetOptions([]string{chosen.ProfileLabel(model.ProfileBalanced), chosen.ProfileLabel(model.ProfileQuality)})
		profile.SetSelected(chosen.ProfileLabel(profileID))
	}
	format.OnChanged = setFormat
	setFormat(s.Format())
	medium.SetSelected(s.Media)
	selected := widget.NewSelect(nil, nil)
	refresh := func() {
		presets, err = u.engine.EncodingPresets()
		if err != nil {
			dialog.ShowError(err, u.win)
			return
		}
		options := []string{}
		for _, p := range presets {
			options = append(options, p.Name)
		}
		selected.SetOptions(options)
		u.loadHeaderPresets()
		u.refreshHeaderSettings()
	}
	selected.OnChanged = func(value string) {
		for _, p := range presets {
			if p.Name == value {
				name.SetText(p.Name)
				format.SetSelected(p.DiscFormat)
				if p.DiscFormat == "" {
					format.SetSelected("DVD")
				}
				standard.SetSelected(p.Standard)
				medium.SetSelected(p.Media)
				chosen := s
				chosen.DiscFormat = p.DiscFormat
				profile.SetSelected(chosen.ProfileLabel(p.EncodingProfile))
			}
		}
	}
	refresh()
	read := func() model.EncodingPreset {
		return model.EncodingPreset{Name: strings.TrimSpace(name.Text), DiscFormat: format.Selected, Standard: standard.Selected, Media: medium.Selected, EncodingProfile: profileID}
	}
	status := widget.NewLabel("")
	status.Wrapping = fyne.TextWrapWord
	scope := fmt.Sprintf("Apply changes to %d selected jobs. Workers are controlled in Settings.", len(ids))
	if len(ids) == 0 {
		scope = "Create presets for jobs or choose defaults for new jobs. Workers are controlled in Settings."
	}
	note := widget.NewLabel(scope)
	note.Wrapping = fyne.TextWrapWord
	save := widget.NewButton("Save preset", func() {
		p := read()
		if err := u.engine.SaveEncodingPreset(p); err != nil {
			dialog.ShowError(err, u.win)
			return
		}
		refresh()
		selected.SetSelected(p.Name)
		status.SetText("Saved " + p.Name)
	})
	remove := widget.NewButton("Delete preset", func() {
		if selected.Selected == "" {
			return
		}
		if err := u.engine.DeleteEncodingPreset(selected.Selected); err != nil {
			dialog.ShowError(err, u.win)
			return
		}
		selected.ClearSelected()
		refresh()
		status.SetText("Preset deleted")
	})
	apply := widget.NewButton(fmt.Sprintf("Apply to %d jobs", len(ids)), func() {
		p := read()
		u.action(func() error {
			err := u.engine.UpdateJobs(ids, p.Apply)
			if err == nil {
				fyne.Do(func() { status.SetText(fmt.Sprintf("Applied to %d jobs", len(ids))) })
			}
			return err
		})
	})
	enabled(apply, len(ids) > 0)
	defaults := widget.NewButton("Use for new jobs", func() {
		p := read()
		u.action(func() error {
			next := u.engine.Snapshot().Settings
			p.Apply(&next)
			err := u.engine.SetSettings("", next)
			if err == nil {
				fyne.Do(func() { status.SetText("Encoding defaults saved for new jobs") })
			}
			return err
		})
	})
	form := widget.NewForm(widget.NewFormItem("Saved preset", selected), widget.NewFormItem("Name", name), widget.NewFormItem("Disc format", format), widget.NewFormItem("Disc capacity", medium), widget.NewFormItem("TV standard", standard), widget.NewFormItem("Encoding profile", profile))
	d := dialog.NewCustom("Encoding presets", "Close", container.NewVBox(note, form, container.NewHBox(save, remove, layout.NewSpacer()), container.NewHBox(defaults, apply), status), u.win)
	d.Resize(fyne.NewSize(650, 480))
	d.Show()
}

func (u *window) menuPresetsDialog() {
	ids := u.headerJobIDs()
	presets, err := u.engine.MenuPresets()
	if err != nil {
		dialog.ShowError(err, u.win)
		return
	}
	selected := widget.NewSelect(nil, nil)
	refresh := func() {
		presets, err = u.engine.MenuPresets()
		if err != nil {
			dialog.ShowError(err, u.win)
			return
		}
		names := []string{}
		for _, p := range presets {
			names = append(names, p.Name)
		}
		selected.SetOptions(names)
		u.loadHeaderPresets()
		u.refreshHeaderSettings()
	}
	refresh()
	status := widget.NewLabel("")
	status.Wrapping = fyne.TextWrapWord
	note := widget.NewLabel(fmt.Sprintf("Assign a saved menu to %d selected jobs, or open the menu editor to create and save presets.", len(ids)))
	note.Wrapping = fyne.TextWrapWord
	get := func() (model.MenuPreset, bool) {
		for _, p := range presets {
			if p.Name == selected.Selected {
				return p, true
			}
		}
		return model.MenuPreset{}, false
	}
	apply := widget.NewButton(fmt.Sprintf("Apply to %d jobs", len(ids)), func() {
		p, ok := get()
		if !ok {
			return
		}
		u.action(func() error {
			err := u.engine.ApplyMenus(ids, p.Style, p.Layout, p.TitlesPerPage)
			if err == nil {
				fyne.Do(func() { status.SetText(fmt.Sprintf("Applied to %d jobs", len(ids))) })
			}
			return err
		})
	})
	enabled(apply, len(ids) > 0)
	defaults := widget.NewButton("Use for new jobs", func() {
		p, ok := get()
		if !ok {
			return
		}
		u.action(func() error { return u.engine.SetMenu("", p.Style, p.Layout, p.TitlesPerPage) })
	})
	remove := widget.NewButton("Delete preset", func() {
		if selected.Selected == "" {
			return
		}
		if err := u.engine.DeleteMenuPreset(selected.Selected); err != nil {
			dialog.ShowError(err, u.win)
			return
		}
		selected.ClearSelected()
		refresh()
	})
	var d *dialog.CustomDialog
	edit := widget.NewButton("Open menu editor…", func() { u.menuSettingsPreset("", nil, selected.Selected) })
	d = dialog.NewCustom("Menu presets", "Close", container.NewVBox(note, selected, container.NewHBox(edit, widget.NewButton("Refresh presets", refresh), remove), container.NewHBox(defaults, apply), status), u.win)
	d.Resize(fyne.NewSize(670, 300))
	d.Show()
}
