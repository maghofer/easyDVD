package ui

import (
	"fmt"
	"math"

	"easydvd/internal/model"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func (u *window) buildStatusBar() fyne.CanvasObject {
	u.queueProgress = &thinProgress{}
	u.queueProgress.ExtendBaseWidget(u.queueProgress)
	u.queuePosition = widget.NewLabel("")
	u.queuePercent = widget.NewLabel("")
	u.footerProgress = container.NewHBox(u.queuePosition, container.NewCenter(u.queueProgress), u.queuePercent)
	u.footerProgress.Hide()
	return container.NewVBox(widget.NewSeparator(), container.NewBorder(nil, nil, nil, u.footerProgress, u.status))
}

func (u *window) refreshStatusProgress() {
	index := -1
	for _, state := range []model.State{model.Running, model.Scanning, model.Paused} {
		for i, j := range u.snapshot.Jobs {
			if j.State == state && (state != model.Paused || j.Progress > 0) {
				index = i
				break
			}
		}
		if index >= 0 {
			break
		}
	}
	if index < 0 || u.snapshot.Clearing {
		u.footerProgress.Hide()
		u.win.Content().Refresh()
		return
	}
	j := u.snapshot.Jobs[index]
	value := max(0, min(1, j.Progress))
	percent := fmt.Sprintf("%.0f%%", math.Round(value*100))
	if j.State == model.Scanning {
		percent = "Scanning"
		value = 0
		if j.Discovered > 0 {
			value = min(1, float64(len(j.Videos))/float64(j.Discovered))
		}
	}
	u.queuePosition.SetText(fmt.Sprintf("Job %d of %d", index+1, len(u.snapshot.Jobs)))
	u.queuePercent.SetText(percent)
	u.queueProgress.value = value
	u.queueProgress.Refresh()
	u.footerProgress.Show()
	if j.Stage != "" && u.mediaReady && !u.mediaChecking && u.snapshot.Error == "" {
		u.status.SetText(j.Stage)
	}
	u.win.Content().Refresh()
}

// A six-pixel track keeps progress in the status bar without a full-height
// progress widget or repeated bars in individual job headers.
type thinProgress struct {
	widget.BaseWidget
	value float64
}

func (p *thinProgress) CreateRenderer() fyne.WidgetRenderer {
	return &thinProgressRenderer{bar: p, track: canvas.NewRectangle(theme.ButtonColor()), fill: canvas.NewRectangle(theme.PrimaryColor())}
}

type thinProgressRenderer struct {
	bar         *thinProgress
	track, fill *canvas.Rectangle
}

func (r *thinProgressRenderer) MinSize() fyne.Size { return fyne.NewSize(180, 6) }
func (r *thinProgressRenderer) Destroy()           {}
func (r *thinProgressRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.track, r.fill}
}
func (r *thinProgressRenderer) Layout(size fyne.Size) {
	r.track.CornerRadius, r.fill.CornerRadius = 3, 3
	r.track.Resize(size)
	r.fill.Resize(fyne.NewSize(size.Width*float32(r.bar.value), size.Height))
}
func (r *thinProgressRenderer) Refresh() {
	r.track.FillColor, r.fill.FillColor = theme.ButtonColor(), theme.PrimaryColor()
	r.Layout(r.bar.Size())
	r.track.Refresh()
	r.fill.Refresh()
}
