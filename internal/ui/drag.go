package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"math"
)

type listDrag struct {
	u             *window
	list          *widget.List
	surface       fyne.CanvasObject
	count, before int
	jobs          bool
	ids           []string
	jobID         string
	line          *canvas.Rectangle
	overlay       *fyne.Container
	rowHeight     float32
}

func (u *window) beginListDrag(list *widget.List, count int, jobs bool) *listDrag {
	d := &listDrag{u: u, list: list, count: count, jobs: jobs, before: -1}
	d.rowHeight = list.CreateItem().MinSize().Height + theme.Padding()
	if jobs {
		d.surface = u.queue
		d.ids = u.headerJobIDs()
	} else {
		d.surface = u.files
		j := u.selectedJob()
		d.jobID = j.ID
		for _, v := range j.Videos {
			if u.selectedFiles[v.ID] {
				d.ids = append(d.ids, v.ID)
			}
		}
	}
	d.line = canvas.NewRectangle(theme.PrimaryColor())
	d.overlay = container.NewWithoutLayout(d.line)
	u.win.Canvas().Overlays().Add(d.overlay)
	return d
}
func (d *listDrag) move(e *fyne.DragEvent) {
	origin := d.u.app.Driver().AbsolutePositionForObject(d.surface)
	y := e.AbsolutePosition.Y - origin.Y
	h := d.list.Size().Height
	offset := d.list.GetScrollOffset()
	if y < 24 {
		d.list.ScrollToOffset(max(0, offset-24))
	} else if y > h-24 {
		d.list.ScrollToOffset(offset + 24)
	}
	row := d.rowHeight
	if row <= 0 {
		return
	}
	d.before = max(0, min(d.count, int(math.Floor(float64((y+d.list.GetScrollOffset())/row)+.5))))
	marker := float32(d.before)*row - d.list.GetScrollOffset()
	marker = max(0, min(h-3, marker))
	d.line.Move(fyne.NewPos(origin.X, origin.Y+marker))
	d.line.Resize(fyne.NewSize(d.list.Size().Width, 3))
	d.line.Refresh()
}
func (d *listDrag) end() {
	d.u.win.Canvas().Overlays().Remove(d.overlay)
	if d.before < 0 {
		return
	}
	d.u.action(func() error {
		if d.jobs {
			return d.u.engine.ReorderJobs(d.ids, d.before)
		}
		return d.u.engine.ReorderVideos(d.jobID, d.ids, d.before)
	})
}
