package ui

import (
	"fmt"

	"easydvd/internal/model"
	"easydvd/internal/platform"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
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
	open := widget.NewButtonWithIcon("Open output", theme.FolderOpenIcon(), func() {
		if j := u.selectedJob(); j != nil {
			path := jobOutputFolder(*j)
			u.action(func() error { return platform.OpenFolder(path) })
		}
	})
	titleRow := container.New(titleEditLayout{title: u.title}, container.NewThemeOverride(u.title, headerTextTheme{size: 14}), u.renameJob)
	info := container.New(tightHeaderLayout{},
		titleRow,
		container.NewThemeOverride(u.source, headerTextTheme{size: 12}),
		container.NewThemeOverride(u.jobOutput, headerTextTheme{size: 12}),
		container.NewThemeOverride(u.jobSummary, headerTextTheme{size: 12}))
	controls := container.NewGridWithRows(2,
		container.NewHBox(layout.NewSpacer(), u.jobWarning),
		container.NewHBox(u.settings, open))
	body := container.New(fixedHeaderLayout{}, container.NewBorder(nil, nil, nil, controls, info))
	u.jobHeader = container.NewVBox(body, widget.NewSeparator())
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
	discs := fmt.Sprintf("~%d DVDs", j.EstimatedDiscs)
	if j.EstimatedDiscs == 1 {
		discs = "~1 DVD"
	}
	if len(j.Discs) > 0 {
		discs = fmt.Sprintf("%d DVDs", len(j.Discs))
		if len(j.Discs) == 1 {
			discs = "1 DVD"
		}
	}
	u.jobSummary.SetText(fmt.Sprintf("%d/%d videos · %s · %s", len(j.ActiveVideos()), len(j.Videos), model.DurationString(j.Duration()), discs))
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
