package ui

import (
	"fmt"
	"image/color"
	"math"

	"easydvd/internal/model"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type jobList struct {
	widget.List
	u *window
}

func (l *jobList) TypedKey(event *fyne.KeyEvent) {
	if event.Name == fyne.KeyDelete {
		l.u.removeSelectedJobs()
		return
	}
	l.List.TypedKey(event)
}

func (l *jobList) TypedShortcut(shortcut fyne.Shortcut) {
	if _, ok := shortcut.(*fyne.ShortcutSelectAll); ok {
		l.u.selectAllJobs()
	}
}

func (u *window) buildJobList() fyne.CanvasObject {
	u.selectedJobs = make(map[string]bool)
	u.queue = &jobList{u: u}
	u.queue.Length = func() int { return len(u.snapshot.Jobs) }
	u.queue.CreateItem = func() fyne.CanvasObject { return newJobTile(u) }
	u.queue.UpdateItem = func(i widget.ListItemID, object fyne.CanvasObject) {
		if i < len(u.snapshot.Jobs) {
			object.(*jobTile).update(u.snapshot.Jobs[i])
		}
	}
	u.queue.HideSeparators = true
	u.queue.ExtendBaseWidget(u.queue)
	u.queue.OnSelected = func(i widget.ListItemID) {
		if i < len(u.snapshot.Jobs) {
			id := u.snapshot.Jobs[i].ID
			u.selectJob(id)
			u.queue.Unselect(i)
		}
	}
	u.jobSelectAll = widget.NewButton("Select all", u.selectAllJobs)
	u.jobSelectAll.Importance = widget.LowImportance
	u.jobSelectionCount = widget.NewLabel("0 selected")
	u.up = widget.NewButtonWithIcon("", theme.MoveUpIcon(), func() { id := u.selected; u.action(func() error { u.engine.Move(id, -1); return nil }) })
	u.down = widget.NewButtonWithIcon("", theme.MoveDownIcon(), func() { id := u.selected; u.action(func() error { u.engine.Move(id, 1); return nil }) })
	u.remove = widget.NewButtonWithIcon("Remove selected", theme.DeleteIcon(), u.removeSelectedJobs)
	u.clear = widget.NewButtonWithIcon("Clear queue", theme.DeleteIcon(), u.clearQueue)
	heading := container.NewBorder(nil, nil, nil, u.clear, widget.NewLabelWithStyle("Queue", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
	selection := container.NewHBox(u.jobSelectAll, layout.NewSpacer(), u.jobSelectionCount)
	return container.NewBorder(container.NewVBox(heading, selection), container.NewHBox(u.up, u.down, layout.NewSpacer(), u.remove), nil, nil, u.queue)
}

func (u *window) selectAllJobs() {
	for _, j := range u.snapshot.Jobs {
		u.selectedJobs[j.ID] = true
	}
	u.win.Canvas().Focus(u.queue)
	u.refreshJobSelection()
	u.queue.Refresh()
}

func (u *window) selectJob(id string) { u.selectJobs(id, selectionModifiers(u.app)) }

func (u *window) selectJobs(id string, modifiers fyne.KeyModifier) {
	ids := make([]string, len(u.snapshot.Jobs))
	target := -1
	for i, j := range u.snapshot.Jobs {
		ids[i] = j.ID
		if j.ID == id {
			target = i
		}
	}
	if !selectItems(ids, u.selectedJobs, &u.jobSelectionAnchor, id, modifiers) {
		return
	}
	u.selected = id
	u.win.Canvas().Focus(u.queue)
	u.queue.Highlight(target)
	u.refresh()
}

func (u *window) refreshJobSelection() {
	valid := make(map[string]bool, len(u.snapshot.Jobs))
	for _, j := range u.snapshot.Jobs {
		valid[j.ID] = true
	}
	for id := range u.selectedJobs {
		if !valid[id] {
			delete(u.selectedJobs, id)
		}
	}
	if !valid[u.jobSelectionAnchor] {
		u.jobSelectionAnchor = ""
	}
	count := len(u.selectedJobs)
	u.jobSelectionCount.SetText(fmt.Sprintf("%d selected", count))
	enabled(u.jobSelectAll, len(u.snapshot.Jobs) > 0 && !u.snapshot.Clearing)
	enabled(u.remove, count > 0 && !u.snapshot.Clearing)
}

func (u *window) removeSelectedJobs() {
	var ids []string
	for _, j := range u.snapshot.Jobs {
		if u.selectedJobs[j.ID] {
			ids = append(ids, j.ID)
		}
	}
	u.removeJobs(ids)
}

func (u *window) removeJobs(ids []string) {
	if len(ids) == 0 || u.snapshot.Clearing {
		return
	}
	active := false
	for _, j := range u.snapshot.Jobs {
		for _, id := range ids {
			if id == j.ID && (u.snapshot.Busy[id] || j.State == model.Running || j.State == model.Scanning) {
				active = true
			}
		}
	}
	remove := func() { u.action(func() error { return u.engine.RemoveJobs(ids) }) }
	if active {
		d := dialog.NewConfirm("Stop and remove jobs?", fmt.Sprintf("Stop active work and remove %d selected jobs? Source and output files will be kept.", len(ids)), func(ok bool) {
			if ok {
				remove()
			}
		}, u.win)
		d.SetConfirmText("Stop and remove")
		d.Show()
	} else {
		remove()
	}
}

// The engine retains Needs attention for recoverable failures. Presentation
// distinguishes a failed processing attempt from files that need user input.
func jobAppearance(j model.Job) (string, fyne.ThemeColorName) {
	if j.State == model.Blocked && (j.Locked || j.Stage == "Folder scan failed") {
		return "Failed", theme.ColorNameError
	}
	for _, v := range j.Videos {
		if v.Error != "" && !v.Excluded && (j.State == model.Ready || j.State == model.Blocked) {
			return "Needs attention", theme.ColorNameWarning
		}
	}
	switch j.State {
	case model.Ready:
		return "Ready", theme.ColorNameForeground
	case model.Running:
		return "Running", theme.ColorNamePrimary
	case model.Complete:
		return "Completed", theme.ColorNameSuccess
	case model.Blocked:
		return "Needs attention", theme.ColorNameWarning
	case model.Scanning:
		return "Scanning", theme.ColorNamePrimary
	default:
		return string(j.State), theme.ColorNameForeground
	}
}

type jobTile struct {
	widget.BaseWidget
	u             *window
	id, fullTitle string
	background    *canvas.Rectangle
	stripe        *canvas.Rectangle
	active        *canvas.Rectangle
	title, state  *canvas.Text
	close         *fileAction
}

func newJobTile(u *window) *jobTile {
	r := &jobTile{u: u, background: canvas.NewRectangle(color.Transparent), stripe: canvas.NewRectangle(color.Transparent), active: canvas.NewRectangle(color.Transparent)}
	r.ExtendBaseWidget(r)
	r.title = canvas.NewText("Folder", theme.ForegroundColor())
	r.title.TextSize, r.title.TextStyle.Bold = 14, true
	r.state = canvas.NewText("Ready", theme.ForegroundColor())
	r.state.TextSize = 12
	r.close = newFileAction(u, theme.CancelIcon(), "Remove job")
	r.active.StrokeWidth, r.active.CornerRadius = 1, 3
	return r
}

func (r *jobTile) Tapped(*fyne.PointEvent) { r.u.selectJob(r.id) }

func (r *jobTile) update(j model.Job) {
	if r.id != j.ID {
		r.close.hideHint()
	}
	r.id, r.fullTitle = j.ID, j.Title
	label, name := jobAppearance(j)
	tint := theme.Color(name)
	r.stripe.FillColor, r.state.Color = tint, tint
	r.title.Color = theme.ForegroundColor()
	r.background.FillColor = withAlpha(tint, 18)
	if r.u.selectedJobs[j.ID] {
		r.background.FillColor = theme.SelectionColor()
	}
	r.active.StrokeColor = color.Transparent
	if j.ID == r.u.selected {
		r.active.StrokeColor = theme.PrimaryColor()
	}
	r.state.Text = fmt.Sprintf("%s · %d videos", label, len(j.Videos))
	if j.State == model.Running || j.State == model.Paused || j.State == model.Complete {
		percent := math.Round(max(0, min(1, j.Progress)) * 100)
		if j.State == model.Complete {
			percent = 100
		}
		r.state.Text += fmt.Sprintf(" · %.0f%%", percent)
	}
	r.close.OnTapped = func() { r.u.removeJobs([]string{j.ID}) }
	enabled(r.close, !r.u.snapshot.Clearing)
	r.Refresh()
}

func withAlpha(c color.Color, alpha uint8) color.Color {
	r, g, b, _ := c.RGBA()
	return color.NRGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: alpha}
}

func (r *jobTile) CreateRenderer() fyne.WidgetRenderer { return &jobTileRenderer{tile: r} }

type jobTileRenderer struct{ tile *jobTile }

func (r *jobTileRenderer) MinSize() fyne.Size { return fyne.NewSize(160, 46) }
func (r *jobTileRenderer) Destroy()           { r.tile.close.hideHint() }
func (r *jobTileRenderer) Objects() []fyne.CanvasObject {
	t := r.tile
	return []fyne.CanvasObject{t.background, t.active, t.stripe, t.title, t.state, t.close}
}
func (r *jobTileRenderer) Layout(size fyne.Size) {
	t := r.tile
	t.background.Resize(size)
	t.active.Resize(size)
	t.stripe.Resize(fyne.NewSize(3, size.Height))
	t.close.Move(fyne.NewPos(size.Width-34, (size.Height-32)/2))
	t.close.Resize(fyne.NewSize(32, 32))
	width := max(0, size.Width-48)
	t.title.Text = elideJobText(t.fullTitle, width, t.title.TextSize, t.title.TextStyle)
	t.title.Move(fyne.NewPos(10, 4))
	t.title.Resize(fyne.NewSize(width, 19))
	t.state.Move(fyne.NewPos(10, 24))
	t.state.Resize(fyne.NewSize(width, 16))
}
func (r *jobTileRenderer) Refresh() {
	r.Layout(r.tile.Size())
	for _, object := range r.Objects() {
		object.Refresh()
	}
}

func elideJobText(text string, width, size float32, style fyne.TextStyle) string {
	if fyne.MeasureText(text, size, style).Width <= width {
		return text
	}
	runes := []rune(text)
	lo, hi := 0, len(runes)
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if fyne.MeasureText(string(runes[:mid])+"…", size, style).Width <= width {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	return string(runes[:lo]) + "…"
}
