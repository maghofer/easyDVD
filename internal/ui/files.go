package ui

import (
	"fmt"
	"image/color"
	"strings"
	"time"

	"easydvd/internal/model"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func filesEditable(j *model.Job) bool {
	return j != nil && j.State != model.Running && j.State != model.Scanning && j.State != model.Complete
}

func (u *window) buildFileList() fyne.CanvasObject {
	u.selectedFiles = make(map[string]bool)
	u.files = widget.NewList(func() int {
		if j := u.selectedJob(); j != nil {
			return len(j.Videos)
		}
		return 0
	}, func() fyne.CanvasObject {
		return newFileRow(u)
	}, func(i widget.ListItemID, object fyne.CanvasObject) {
		if j := u.selectedJob(); j != nil && i < len(j.Videos) {
			object.(*fileRow).update(j, i)
		}
	})
	u.files.HideSeparators = true
	// Checkboxes hold the multi-selection; retain keyboard selection through the list.
	u.files.OnSelected = func(i widget.ListItemID) {
		if j := u.selectedJob(); j != nil && i < len(j.Videos) {
			u.setFileSelected(j.Videos[i].ID, !u.selectedFiles[j.Videos[i].ID])
			u.files.Unselect(i)
		}
	}
	u.selectAll = widget.NewCheck("Select all", func(checked bool) {
		if j := u.selectedJob(); j != nil {
			for _, v := range j.Videos {
				if checked {
					u.selectedFiles[v.ID] = true
				} else {
					delete(u.selectedFiles, v.ID)
				}
			}
		}
		u.refreshFileSelection()
		u.files.Refresh()
	})
	u.selectionCount = widget.NewLabel("0 selected")
	u.excludeSelected = widget.NewButtonWithIcon("Exclude selected", theme.VisibilityOffIcon(), func() { u.setSelectedExcluded(true) })
	u.includeSelected = widget.NewButtonWithIcon("Include selected", theme.VisibilityIcon(), func() { u.setSelectedExcluded(false) })
	bar := container.NewHBox(u.selectAll, u.selectionCount, layout.NewSpacer(), u.excludeSelected, u.includeSelected)
	return container.NewBorder(bar, nil, nil, nil, u.files)
}

func (u *window) setFileSelected(id string, selected bool) {
	shift := false
	if driver, ok := u.app.Driver().(desktop.Driver); ok {
		shift = driver.CurrentKeyModifiers()&fyne.KeyModifierShift != 0
	}
	u.selectFileRange(id, selected, shift)
}

func (u *window) selectFileRange(id string, selected, shift bool) {
	if shift && u.selectionAnchor != "" {
		if j := u.selectedJob(); j != nil {
			anchor, target := -1, -1
			for i, v := range j.Videos {
				if v.ID == u.selectionAnchor {
					anchor = i
				}
				if v.ID == id {
					target = i
				}
			}
			if anchor >= 0 && target >= 0 {
				for i := min(anchor, target); i <= max(anchor, target); i++ {
					// Shift always selects the inclusive range, even if the endpoint was selected.
					u.selectedFiles[j.Videos[i].ID] = true
				}
				u.refreshFileSelection()
				u.files.Refresh()
				return
			}
		}
	}
	u.selectionAnchor = id
	if selected {
		u.selectedFiles[id] = true
	} else {
		delete(u.selectedFiles, id)
	}
	u.refreshFileSelection()
	u.files.Refresh()
}

func (u *window) refreshFileSelection() {
	j := u.selectedJob()
	jobID := ""
	if j != nil {
		jobID = j.ID
	}
	if u.selectionJob != jobID {
		u.selectionJob = jobID
		u.selectionAnchor = ""
		clear(u.selectedFiles)
		u.files.UnselectAll()
		u.files.ScrollToTop()
	}
	count, included, excluded, total := 0, 0, 0, 0
	valid := make(map[string]bool)
	if j != nil {
		total = len(j.Videos)
		for _, v := range j.Videos {
			valid[v.ID] = true
			if u.selectedFiles[v.ID] {
				count++
				if v.Excluded {
					excluded++
				} else {
					included++
				}
			}
		}
	}
	for id := range u.selectedFiles {
		if !valid[id] {
			delete(u.selectedFiles, id)
		}
	}
	u.selectionCount.SetText(fmt.Sprintf("%d selected", count))
	change := u.selectAll.OnChanged
	u.selectAll.OnChanged = nil
	u.selectAll.SetChecked(total > 0 && count == total)
	u.selectAll.OnChanged = change
	enabled(u.selectAll, total > 0)
	enabled(u.excludeSelected, filesEditable(j) && !u.snapshot.Clearing && included > 0)
	enabled(u.includeSelected, filesEditable(j) && !u.snapshot.Clearing && excluded > 0)
}

func (u *window) setSelectedExcluded(excluded bool) {
	j := u.selectedJob()
	if !filesEditable(j) || u.snapshot.Clearing {
		return
	}
	var ids []string
	for _, v := range j.Videos {
		if u.selectedFiles[v.ID] {
			ids = append(ids, v.ID)
		}
	}
	jobID := j.ID
	u.action(func() error { return u.engine.SetExcluded(jobID, ids, excluded) })
}

func (u *window) editFileTitle(jobID string, v model.Video) {
	entry := widget.NewEntry()
	entry.SetText(v.Label)
	entry.Validator = func(value string) error {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("enter a title")
		}
		return nil
	}
	entry.AlwaysShowValidationError = true
	source := widget.NewLabel(v.Relative + "\nOriginal title: " + v.OriginalLabel())
	source.Wrapping = fyne.TextWrapWord
	d := dialog.NewForm("Edit video title", "Save", "Cancel", []*widget.FormItem{
		widget.NewFormItem("Source", source), widget.NewFormItem("Title", entry),
	}, func(ok bool) {
		if ok {
			label := strings.TrimSpace(entry.Text)
			u.action(func() error { return u.engine.Edit(jobID, "", v.ID, label, nil) })
		}
	}, u.win)
	d.Resize(fyne.NewSize(580, 220))
	d.Show()
	u.win.Canvas().Focus(entry)
}

func (u *window) showFileIssue(v model.Video) {
	message := widget.NewLabel(v.Relative + "\n\n" + v.Error)
	message.Wrapping = fyne.TextWrapWord
	d := dialog.NewCustom("File needs attention", "Close", container.NewVScroll(message), u.win)
	d.Resize(fyne.NewSize(580, 300))
	d.Show()
}

type fileRow struct {
	widget.BaseWidget
	u                             *window
	id                            string
	index                         int
	selected                      bool
	background                    *canvas.Rectangle
	selectFile                    *widget.Check
	title, state, duration        *widget.Label
	warning, edit, revert, toggle *fileAction
	content                       fyne.CanvasObject
}

func newFileRow(u *window) *fileRow {
	r := &fileRow{u: u, background: canvas.NewRectangle(color.Transparent)}
	r.ExtendBaseWidget(r)
	r.selectFile = widget.NewCheck("", nil)
	r.title = widget.NewLabel("Video title")
	r.title.Truncation = fyne.TextTruncateEllipsis
	r.state = widget.NewLabel("")
	r.duration = widget.NewLabel("00:00:00")
	r.warning = newFileAction(u, theme.NewWarningThemedResource(theme.WarningIcon()), "Show file problem")
	r.edit = newFileAction(u, theme.DocumentCreateIcon(), "Edit title")
	r.revert = newFileAction(u, theme.ContentUndoIcon(), "Restore original title")
	r.toggle = newFileAction(u, theme.VisibilityIcon(), "Exclude file from DVD")
	// Keep a fixed warning slot so actions stay aligned across healthy and failed rows.
	warningSlot := container.NewGridWrap(r.warning.MinSize(), r.warning)
	actions := container.NewHBox(r.state, r.duration, warningSlot, r.edit, r.revert, r.toggle)
	r.content = container.NewStack(r.background, container.NewBorder(nil, nil, r.selectFile, actions, r.title))
	return r
}

func (r *fileRow) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(r.content)
}

func (r *fileRow) Tapped(*fyne.PointEvent) {
	r.u.setFileSelected(r.id, !r.u.selectedFiles[r.id])
}

func (r *fileRow) update(j *model.Job, index int) {
	v := j.Videos[index]
	if r.id != v.ID {
		for _, b := range []*fileAction{r.warning, r.edit, r.revert, r.toggle} {
			b.hideHint()
		}
	}
	r.id, r.index, r.selected = v.ID, index, r.u.selectedFiles[v.ID]
	r.selectFile.OnChanged = nil
	r.selectFile.SetChecked(r.selected)
	r.selectFile.OnChanged = func(checked bool) { r.u.setFileSelected(v.ID, checked) }
	r.title.SetText(v.Label)
	edited := v.Label != v.OriginalLabel()
	r.title.TextStyle = fyne.TextStyle{Italic: edited, Strikethrough: v.Excluded}
	r.title.Importance = widget.MediumImportance
	var states []string
	if edited {
		states = append(states, "Edited")
	}
	if v.Excluded {
		states = append(states, "Excluded")
	}
	r.state.SetText(strings.Join(states, " · "))
	duration := model.DurationString(v.Duration)
	if v.Duration <= 0 {
		duration = "—"
	}
	r.duration.SetText(duration)
	if v.Error != "" {
		r.warning.Show()
	} else {
		r.warning.Hide()
	}
	r.warning.OnTapped = func() { r.u.showFileIssue(v) }
	jobID := j.ID
	r.edit.OnTapped = func() { r.u.editFileTitle(jobID, v) }
	r.revert.OnTapped = func() {
		r.u.action(func() error { return r.u.engine.Edit(jobID, "", v.ID, v.OriginalLabel(), nil) })
	}
	r.toggle.SetIcon(theme.VisibilityIcon())
	r.toggle.hint = "Exclude file from DVD"
	if v.Excluded {
		r.toggle.SetIcon(theme.VisibilityOffIcon())
		r.toggle.hint = "Include file in DVD"
	}
	r.toggle.OnTapped = func() {
		r.u.action(func() error { return r.u.engine.SetExcluded(jobID, []string{v.ID}, !v.Excluded) })
	}
	enabled(r.edit, filesEditable(j) && !r.u.snapshot.Clearing)
	enabled(r.revert, filesEditable(j) && !r.u.snapshot.Clearing && edited)
	enabled(r.toggle, filesEditable(j) && !r.u.snapshot.Clearing)
	r.Refresh()
}

func (r *fileRow) Refresh() {
	r.background.FillColor = color.Transparent
	if r.index%2 == 1 {
		r.background.FillColor = theme.HoverColor()
	}
	if r.selected {
		r.background.FillColor = theme.SelectionColor()
	}
	r.BaseWidget.Refresh()
}

// fileAction adds accessible names and hover help to icon-only buttons.
// The help contains no interactive objects, so it does not intercept clicks.
type fileAction struct {
	widget.Button
	u       *window
	hint    string
	help    *fyne.Container
	version uint64
}

func newFileAction(u *window, icon fyne.Resource, hint string) *fileAction {
	b := &fileAction{u: u, hint: hint}
	b.Icon = icon
	b.Importance = widget.LowImportance
	b.ExtendBaseWidget(b)
	return b
}

func (b *fileAction) AccessibilityLabel() string { return b.hint }

func (b *fileAction) MouseIn(event *desktop.MouseEvent) {
	b.Button.MouseIn(event)
	b.version++
	version := b.version
	time.AfterFunc(550*time.Millisecond, func() {
		fyne.Do(func() {
			if version != b.version || !b.Visible() || b.u.win.Canvas().Overlays().Top() != nil {
				return
			}
			select {
			case <-b.u.closed:
				return
			default:
			}
			text := widget.NewLabel(b.hint)
			tip := container.NewStack(canvas.NewRectangle(theme.OverlayBackgroundColor()), text)
			tip.Resize(tip.MinSize())
			pos := fyne.CurrentApp().Driver().AbsolutePositionForObject(b)
			pos.Y += b.Size().Height + 2
			pos.X = max(0, min(pos.X, b.u.win.Canvas().Size().Width-tip.MinSize().Width))
			pos.Y = max(0, min(pos.Y, b.u.win.Canvas().Size().Height-tip.MinSize().Height))
			tip.Move(pos)
			b.help = tip
			b.u.fileHints.Add(b.help)
		})
	})
}

func (b *fileAction) hideHint() {
	b.version++
	if b.help != nil {
		b.u.fileHints.Remove(b.help)
		b.help = nil
	}
}

func (b *fileAction) MouseOut() {
	b.hideHint()
	b.Button.MouseOut()
}

func (b *fileAction) Tapped(event *fyne.PointEvent) {
	b.hideHint()
	b.Button.Tapped(event)
}

func (b *fileAction) Hide() {
	b.hideHint()
	b.Button.Hide()
}
