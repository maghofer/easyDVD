package ui

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"easydvd"
	"easydvd/internal/release"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func (u *window) updateSettings() fyne.CanvasObject {
	status := widget.NewLabel("Checks GitHub for stable releases only when you click. No automatic checks.")
	status.Wrapping = fyne.TextWrapWord
	open := widget.NewButtonWithIcon("Open release page", theme.NavigateNextIcon(), nil)
	open.Hide()
	var check *widget.Button
	check = widget.NewButton("Check for updates", func() {
		check.Disable()
		open.Hide()
		status.SetText("Checking GitHub…")
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			r, err := release.Check(ctx, easydvd.Repository(), easydvd.Version())
			fyne.Do(func() {
				check.Enable()
				if err != nil {
					status.SetText("Update check failed: " + err.Error())
					return
				}
				if r.NoRelease {
					status.SetText("No public stable release is available yet (or the repository is unavailable).")
					return
				}
				if !r.Available {
					status.SetText("You are up to date. Latest stable release: " + r.Version)
					return
				}
				status.SetText(fmt.Sprintf("easyDVD %s is available. Current version: %s.", r.Version, easydvd.Version()))
				open.OnTapped = func() {
					link, err := url.Parse(r.URL)
					if err == nil {
						err = u.app.OpenURL(link)
					}
					if err != nil {
						dialog.ShowError(err, u.win)
					}
				}
				open.Show()
			})
		}()
	})
	return container.NewVBox(widget.NewSeparator(), widget.NewLabel("easyDVD "+easydvd.Version()), container.NewHBox(check, open), status)
}
