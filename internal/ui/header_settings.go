package ui

import (
	"fmt"

	"easydvd/internal/model"
	"easydvd/internal/platform"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

func (u *window) headerJobIDs() []string {
	var ids []string
	for _, j := range u.snapshot.Jobs {
		if u.selectedJobs[j.ID] {
			ids = append(ids, j.ID)
		}
	}
	if len(ids) == 0 && u.selectedJob() != nil {
		ids = append(ids, u.selected)
	}
	return ids
}
func (u *window) patchSelected(patch func(*model.Settings)) {
	if u.headerUpdating {
		return
	}
	ids := u.headerJobIDs()
	u.action(func() error { return u.engine.UpdateJobs(ids, patch) })
}
func (u *window) patchPlayback(noMenu bool) {
	if u.headerUpdating {
		return
	}
	ids := u.headerJobIDs()
	u.action(func() error { return u.engine.SetPlayback(ids, noMenu) })
}
func (u *window) changeJobFormat(format string) {
	u.patchSelected(func(s *model.Settings) {
		s.DiscFormat = format
		if s.IsBluray() {
			if s.Media != "BD-25" && s.Media != "BD-50" {
				s.Media = "BD-25"
			}
		} else if s.Media != "DVD-5" && s.Media != "DVD-9" {
			s.Media = "DVD-5"
		}
	})
}
func (u *window) refreshHeaderSettings() {
	u.headerUpdating = true
	defer func() { u.headerUpdating = false }()
	ids := u.headerJobIDs()
	if len(ids) == 0 {
		return
	}
	var jobs []model.Job
	editable := !u.snapshot.Clearing
	for _, j := range u.snapshot.Jobs {
		for _, id := range ids {
			if j.ID == id {
				jobs = append(jobs, j)
				editable = editable && !j.Locked && j.State != model.Scanning && !u.snapshot.Busy[id]
			}
		}
	}
	first := jobs[0].Settings
	format := first.Format()
	mixedFormat := false
	for _, j := range jobs {
		mixedFormat = mixedFormat || j.Settings.Format() != format
	}
	u.headerScope.SetText("Job settings")
	u.headerScope.Hide()
	if len(ids) > 1 {
		u.headerScope.SetText(fmt.Sprintf("Editing %d jobs", len(ids)))
		u.headerScope.Show()
	}
	u.dvdButton.Importance = widget.MediumImportance
	u.blurayButton.Importance = widget.MediumImportance
	if !mixedFormat {
		if first.IsBluray() {
			u.blurayButton.Importance = widget.HighImportance
		} else {
			u.dvdButton.Importance = widget.HighImportance
		}
	}
	u.dvdButton.Refresh()
	u.blurayButton.Refresh()
	options := []string{"DVD-5", "DVD-9"}
	if first.IsBluray() {
		options = []string{"BD-25", "BD-50"}
	}
	u.headerMedia.SetOptions(options)
	common := func(get func(model.Settings) string) string {
		v := get(first)
		for _, j := range jobs {
			if get(j.Settings) != v {
				return ""
			}
		}
		return v
	}
	set := func(w *widget.Select, value string) {
		if value == "" {
			w.ClearSelected()
		} else {
			w.SetSelected(value)
		}
	}
	u.headerMedia.SetSelected(common(func(s model.Settings) string { return s.Media }))
	u.headerStandard.SetSelected(common(func(s model.Settings) string { return s.Standard }))
	u.headerPlayback.SetSelected(common(func(s model.Settings) string {
		if s.NoMenu {
			return "Autoplay"
		}
		return "Menu"
	}))
	u.refreshHeaderPresetOptions(first, mixedFormat)
	u.headerEncodingPreset.PlaceHolder = "Encoding: mixed / custom"
	set(u.headerEncodingPreset, common(u.encodingPresetName))
	u.headerMenuPreset.PlaceHolder = "Menu: mixed / custom"
	set(u.headerMenuPreset, common(u.menuPresetName))
	for _, w := range []fyne.Disableable{u.dvdButton, u.blurayButton, u.headerMedia, u.headerStandard, u.headerPlayback, u.headerEncodingPreset, u.headerMenuPreset, u.editOutput} {
		enabled(w, editable)
	}
	u.headerMedia.Disable()
	if editable && !mixedFormat {
		u.headerMedia.Enable()
	}
	if first.IsBluray() && !mixedFormat {
		u.headerStandard.Hide()
		u.headerStandardGroup.Hide()
	} else {
		u.headerStandard.Show()
		u.headerStandardGroup.Show()
	}
	enabled(u.headerStandard, editable && !mixedFormat)
	enabled(u.headerMenuPreset, editable && u.headerPlayback.Selected == "Menu")
	mixedOutput := false
	for _, j := range jobs {
		mixedOutput = mixedOutput || j.Settings.OutputDir != first.OutputDir
	}
	if len(ids) > 1 {
		value := first.OutputDir
		if mixedOutput {
			value = "Mixed folders"
		}
		u.jobOutput.SetText("Output for selected jobs: " + value)
	}
}

func (u *window) changeOutputFolder() {
	if u.editOutput.Disabled() {
		return
	}
	ids := u.headerJobIDs()
	if len(ids) == 0 {
		return
	}
	for _, j := range u.snapshot.Jobs {
		for _, id := range ids {
			if j.ID == id && (j.Locked || u.snapshot.Busy[id]) {
				return
			}
		}
	}
	// The scope is captured before the native picker opens.
	u.action(func() error {
		paths, err := platform.PickFolders(false)
		if err != nil || len(paths) == 0 {
			return err
		}
		return u.engine.UpdateJobs(ids, func(s *model.Settings) { s.OutputDir = paths[0] })
	})
}

func (u *window) loadHeaderPresets() {
	if u.engine == nil {
		return
	}
	encoding, err := u.engine.EncodingPresets()
	if err == nil {
		u.headerEncodingPresets = encoding
	}
	menus, err := u.engine.MenuPresets()
	if err == nil {
		u.headerMenuPresets = menus
	}
}

func (u *window) refreshHeaderPresetOptions(s model.Settings, mixedFormat bool) {
	encoding := []string{s.ProfileLabel(model.ProfileBalanced), s.ProfileLabel(model.ProfileQuality)}
	if mixedFormat {
		encoding = []string{model.ProfileLabel(model.ProfileBalanced), model.ProfileLabel(model.ProfileQuality)}
	}
	for _, preset := range u.headerEncodingPresets {
		encoding = append(encoding, "Preset: "+preset.Name)
	}
	u.headerEncodingPreset.SetOptions(encoding)
	menus := []string{"Default menu"}
	for _, preset := range u.headerMenuPresets {
		menus = append(menus, "Preset: "+preset.Name)
	}
	u.headerMenuPreset.SetOptions(menus)
}

func (u *window) encodingPresetName(s model.Settings) string {
	for _, p := range u.headerEncodingPresets {
		preset := s
		p.Apply(&preset)
		if preset.Format() == s.Format() && preset.Media == s.Media && preset.Standard == s.Standard && preset.Profile() == s.Profile() {
			return "Preset: " + p.Name
		}
	}
	return s.ProfileLabel(s.Profile())
}

func (u *window) menuPresetName(s model.Settings) string {
	for _, p := range u.headerMenuPresets {
		if p.Style == s.Menu && p.Layout == s.Layout && p.TitlesPerPage == s.TitlesPerPage {
			return "Preset: " + p.Name
		}
	}
	d := u.snapshot.Settings
	if s.Menu == d.Menu && s.Layout == d.Layout && s.TitlesPerPage == d.TitlesPerPage {
		return "Default menu"
	}
	return ""
}

func (u *window) chooseHeaderEncodingPreset(name string) {
	if u.headerUpdating {
		return
	}
	for _, p := range u.headerEncodingPresets {
		if name == "Preset: "+p.Name {
			u.patchSelected(p.Apply)
			return
		}
	}
	for _, profile := range []string{model.ProfileBalanced, model.ProfileQuality} {
		if name == model.ProfileLabel(profile) || name == (model.Settings{DiscFormat: "Blu-ray"}).ProfileLabel(profile) {
			u.patchSelected(func(s *model.Settings) { s.EncodingProfile = profile })
			return
		}
	}
}

func (u *window) chooseHeaderMenuPreset(name string) {
	if u.headerUpdating {
		return
	}
	var preset model.MenuPreset
	if name == "Default menu" {
		s := u.snapshot.Settings
		preset = model.MenuPreset{Style: s.Menu, Layout: s.Layout, TitlesPerPage: s.TitlesPerPage}
	} else {
		found := false
		for _, p := range u.headerMenuPresets {
			if name == "Preset: "+p.Name {
				preset, found = p, true
				break
			}
		}
		if !found {
			return
		}
	}
	ids := u.headerJobIDs()
	u.action(func() error { return u.engine.ApplyMenus(ids, preset.Style, preset.Layout, preset.TitlesPerPage) })
}
