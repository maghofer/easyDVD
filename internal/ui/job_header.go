package ui

import (
	"fmt"

	"easydvd/internal/model"
	"easydvd/internal/platform"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func (u *window) buildJobHeader() *fyne.Container {
	u.title = widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	u.title.Truncation = fyne.TextTruncateEllipsis
	u.jobOutput = widget.NewLabel("")
	u.jobOutput.Truncation = fyne.TextTruncateEllipsis
	u.jobSummary = widget.NewLabel("")
	u.jobSummary.Truncation = fyne.TextTruncateEllipsis
	u.source = widget.NewLabel("")
	u.source.Truncation = fyne.TextTruncateEllipsis
	u.jobWarning = widget.NewButtonWithIcon("Needs attention", theme.NewWarningThemedResource(theme.WarningIcon()), u.jobDetails)
	u.jobWarning.Importance = widget.LowImportance
	u.jobWarning.Hide()
	u.renameJob = newFileAction(u, theme.DocumentCreateIcon(), "Edit job title")
	u.renameJob.OnTapped = u.rename
	u.settings = widget.NewButton("Job settings", func() {
		if j := u.selectedJob(); j != nil {
			u.settingsDialog(j.ID, j.Settings)
		}
	})
	u.settings.Hide()
	u.editOutput = newFileAction(u, theme.DocumentCreateIcon(), "Change output folder")
	u.editOutput.OnTapped = u.changeOutputFolder
	u.openOutput = newFileAction(u, theme.FolderOpenIcon(), "Open output folder")
	u.openOutput.OnTapped = func() {
		if j := u.selectedJob(); j != nil {
			path := jobOutputFolder(*j)
			u.action(func() error { return platform.OpenFolder(path) })
		}
	}
	u.headerScope = widget.NewLabel("")
	u.headerScope.TextStyle.Bold = true
	titleRow := container.New(titleEditLayout{title: u.title}, container.NewThemeOverride(u.title, headerTextTheme{size: 14}), u.renameJob)
	outputRow := container.New(outputPathLayout{label: u.jobOutput}, container.NewThemeOverride(u.jobOutput, headerTextTheme{size: 12}), u.editOutput, u.openOutput)
	info := container.New(tightHeaderLayout{},
		container.NewBorder(nil, nil, nil, container.NewHBox(u.headerScope, u.jobWarning), titleRow),
		container.New(sourceSummaryLayout{summary: u.jobSummary}, container.NewThemeOverride(u.source, headerTextTheme{size: 12}), container.NewThemeOverride(u.jobSummary, headerTextTheme{size: 12})),
		outputRow)
	u.dvdButton = newFormatLogoButton(dvdLogo, "dvd-logo.svg", func() { u.changeJobFormat("DVD") })
	u.blurayButton = newFormatLogoButton(blurayLogo, "bluray-logo.svg", func() { u.changeJobFormat("Blu-ray") })
	u.headerMedia = newHeaderButtons([]string{"DVD-5", "DVD-9"}, func(v string) { u.patchSelected(func(s *model.Settings) { s.Media = v }) })
	u.headerStandard = newHeaderButtons([]string{"PAL", "NTSC"}, func(v string) { u.patchSelected(func(s *model.Settings) { s.Standard = v }) })
	u.headerPlayback = newHeaderButtons([]string{"Menu", "Autoplay"}, func(v string) { u.patchPlayback(v == "Autoplay") })
	u.headerEncodingPreset = widget.NewSelect(nil, u.chooseHeaderEncodingPreset)
	u.headerMenuPreset = widget.NewSelect(nil, u.chooseHeaderMenuPreset)
	u.loadHeaderPresets()
	formats := container.NewCenter(container.NewHBox(u.dvdButton, u.blurayButton))
	body := container.NewBorder(nil, nil, nil, formats, info)
	u.headerStandardGroup = container.NewHBox(widget.NewSeparator(), u.headerStandard)
	choices := container.NewHBox(u.headerMedia, u.headerStandardGroup, widget.NewSeparator(), u.headerPlayback)
	presets := container.NewGridWithColumns(2, u.headerEncodingPreset, u.headerMenuPreset)
	settingsRow := container.NewBorder(nil, nil, choices, nil, presets)
	u.jobHeader = container.NewVBox(body, container.NewThemeOverride(settingsRow, headerControlsTheme{headerTextTheme{size: 13}}), widget.NewSeparator())
	return u.jobHeader
}

func (u *window) refreshJobHeader(j *model.Job) {
	if j == nil {
		return
	}
	u.title.SetText(j.Title)
	enabled(u.renameJob, filesEditable(j) && !u.snapshot.Clearing)
	u.source.SetText(j.Folder)
	u.jobOutput.SetText("Output: " + jobOutputFolder(*j))
	u.refreshHeaderSettings()
	discs := fmt.Sprintf("~%d %s discs", j.EstimatedDiscs, j.Settings.DiscLabel())
	if j.EstimatedDiscs == 1 {
		discs = "~1 " + j.Settings.DiscLabel() + " disc"
	}
	if len(j.Discs) > 0 {
		discs = fmt.Sprintf("%d %s discs", len(j.Discs), j.Settings.DiscLabel())
		if len(j.Discs) == 1 {
			discs = "1 " + j.Settings.DiscLabel() + " disc"
		}
	}
	u.jobSummary.SetText(fmt.Sprintf("%d/%d videos · %s · %s", len(j.ActiveVideos()), len(j.Videos), model.DurationString(j.Duration()), discs))
	if j.Settings.IsBluray() {
		u.jobSummary.SetText(u.jobSummary.Text + " · experimental")
	}
	issues := 0
	for _, v := range j.Videos {
		if v.Error != "" && !v.Excluded {
			issues++
		}
	}
	if issues > 0 || j.Error != "" || j.State == model.Blocked {
		if issues == 1 {
			u.jobWarning.SetText("1 file issue")
		} else if issues > 1 {
			u.jobWarning.SetText(fmt.Sprintf("%d file issues", issues))
		} else {
			u.jobWarning.SetText("Job issue")
		}
		u.jobWarning.Show()
	} else {
		u.jobWarning.Hide()
	}
	u.details.Refresh()
}

func jobOutputFolder(j model.Job) string {
	if j.OutputPath != "" {
		return j.OutputPath
	}
	return j.Settings.OutputDir
}

func (u *window) clearQueue() {
	if len(u.snapshot.Jobs) == 0 || u.snapshot.Clearing {
		return
	}
	d := dialog.NewConfirm("Clear queue?", fmt.Sprintf("Remove all %d jobs from the queue? Active scanning and processing will stop. Completed output files will be kept.", len(u.snapshot.Jobs)), func(ok bool) {
		if ok {
			u.clear.Disable()
			u.start.Disable()
			u.action(u.engine.ClearQueue)
		}
	}, u.win)
	d.SetConfirmText("Clear queue")
	d.Show()
}
