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
	u.jobState = widget.NewLabel("")
	u.jobSummary = widget.NewLabel("")
	u.jobSummary.Truncation = fyne.TextTruncateEllipsis
	u.source = widget.NewLabel("")
	u.source.Truncation = fyne.TextTruncateEllipsis
	u.meta = widget.NewLabel("")
	u.meta.Wrapping = fyne.TextWrapWord
	u.stage = widget.NewLabel("")
	u.stage.Wrapping = fyne.TextWrapWord
	u.activity = widget.NewLabel("")
	u.activity.Truncation = fyne.TextTruncateEllipsis
	u.progress = widget.NewProgressBar()
	u.progress.Hide()
	u.scanProgress = widget.NewProgressBarInfinite()
	u.scanProgress.Stop()
	u.scanProgress.Hide()
	u.activity.Hide()
	u.headerToggle = widget.NewButtonWithIcon("More info", theme.MoveDownIcon(), func() {
		u.setHeaderExpanded(!u.headerExpanded)
	})
	u.headerToggle.Importance = widget.LowImportance
	u.jobWarning = widget.NewButtonWithIcon("Needs attention", theme.NewWarningThemedResource(theme.WarningIcon()), u.jobDetails)
	u.jobWarning.Importance = widget.LowImportance
	u.jobWarning.Hide()
	rename := widget.NewButtonWithIcon("Set title", theme.DocumentCreateIcon(), u.rename)
	u.settings = widget.NewButton("Job settings", func() {
		if j := u.selectedJob(); j != nil {
			u.settingsDialog(j.ID, j.Settings)
		}
	})
	u.preview = widget.NewButtonWithIcon("Menu", theme.SettingsIcon(), func() {
		if j := u.selectedJob(); j != nil {
			u.menuSettings(j.ID)
		}
	})
	open := widget.NewButtonWithIcon("Open output", theme.FolderOpenIcon(), func() {
		if j := u.selectedJob(); j != nil {
			path := j.OutputPath
			if path == "" {
				path = j.Settings.OutputDir
			}
			u.action(func() error { return platform.OpenFolder(path) })
		}
	})
	u.headerInfo = container.NewVBox(u.source, u.meta, u.stage, container.NewHBox(rename, u.settings))
	u.headerInfo.Hide()
	titleRow := container.NewBorder(nil, nil, nil, container.NewHBox(u.jobState, u.jobWarning, u.headerToggle), u.title)
	summaryRow := container.NewBorder(nil, nil, nil, container.NewHBox(u.preview, open), u.jobSummary)
	u.jobHeader = container.NewVBox(titleRow, summaryRow, u.activity, u.progress, u.scanProgress, u.headerInfo, widget.NewSeparator())
	return u.jobHeader
}

func (u *window) setHeaderExpanded(expanded bool) {
	u.headerExpanded = expanded
	if expanded {
		u.headerInfo.Show()
		u.headerToggle.SetText("Less info")
		u.headerToggle.SetIcon(theme.MoveUpIcon())
	} else {
		u.headerInfo.Hide()
		u.headerToggle.SetText("More info")
		u.headerToggle.SetIcon(theme.MoveDownIcon())
	}
	if u.details != nil {
		u.details.Refresh()
	}
}

func (u *window) refreshJobHeader(j *model.Job) {
	if j == nil {
		u.headerJob = ""
		u.setHeaderExpanded(false)
		u.progress.Hide()
		u.scanProgress.Stop()
		u.scanProgress.Hide()
		u.activity.Hide()
		return
	}
	if u.headerJob != j.ID {
		u.headerJob = j.ID
		u.setHeaderExpanded(false)
	}
	u.title.SetText(j.Title)
	u.jobState.SetText(string(j.State))
	u.source.SetText(j.Folder)
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
	u.meta.SetText(fmt.Sprintf("%d included / %d inspected · %s · %s · %s\n%s · %s", len(j.ActiveVideos()), len(j.Videos), model.DurationString(j.Duration()), j.Settings.Standard, j.Settings.Media, discs, j.Settings.Packing))
	message := j.Stage
	if j.Error != "" {
		message += "\n" + j.Error
	}
	if runes := []rune(message); len(runes) > 300 {
		message = string(runes[:300]) + "… (see Details)"
	}
	u.stage.SetText(message)
	if message == "" {
		u.stage.Hide()
	} else {
		u.stage.Show()
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
	u.progress.SetValue(j.Progress)
	if j.State == model.Running || (j.State == model.Paused && j.Progress > 0) {
		u.progress.Show()
	} else {
		u.progress.Hide()
	}
	if j.State == model.Scanning {
		u.scanProgress.Show()
		u.scanProgress.Start()
	} else {
		u.scanProgress.Stop()
		u.scanProgress.Hide()
	}
	if (j.State == model.Scanning || j.State == model.Running) && j.Stage != "" {
		u.activity.SetText(j.Stage)
		u.activity.Show()
	} else {
		u.activity.Hide()
	}
	u.details.Refresh()
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
