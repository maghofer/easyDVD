package ui

import (
	"context"
	"fmt"
	"os"
	"time"

	"easydvd/internal/media"
	"easydvd/internal/model"
	"easydvd/internal/platform"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

func (u *window) ensureQuality() bool {
	for _, job := range u.snapshot.Jobs {
		if job.State == model.Complete || job.State == model.Canceled {
			continue
		}
		if !job.Settings.IsBluray() && job.Settings.Profile() == model.ProfileQuality {
			if err := u.engine.Tools().RequireProfile(job.Settings); err != nil {
				u.showQualitySetup()
				return false
			}
		}
	}
	return true
}

func (u *window) showQualitySetup() {
	u.openQualitySetup(false)
}

// Called after media discovery/setup, so a late discovery result cannot replace
// the downloaded encoder. Existing and explicitly selected runtimes are kept.
func (u *window) setupQualityOnLaunch() {
	snapshot := u.engine.Snapshot()
	if snapshot.Settings.IsBluray() {
		needsDVD := false
		for _, job := range snapshot.Jobs {
			if !job.Settings.IsBluray() && job.Settings.Profile() == model.ProfileQuality && job.State != model.Complete && job.State != model.Canceled {
				needsDVD = true
				break
			}
		}
		if !needsDVD {
			return
		}
	}
	t := u.engine.Tools()
	if os.Getenv("EASYDVD_HCENC") == "" && t.HCenc == "" && t.FFMS2 != "" && t.AviSynth != "" && len(u.engine.Snapshot().Busy) == 0 {
		u.openQualitySetup(true)
	}
}

func (u *window) openQualitySetup(automatic bool) {
	if u.qualityDialog != nil {
		return
	}
	if len(u.engine.Snapshot().Busy) != 0 {
		dialog.ShowInformation("Quality (hcenc) tools are in use", "Wait for scanning and processing to stop before changing the encoder.", u.win)
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		select {
		case <-u.closed:
			cancel()
		case <-ctx.Done():
		}
	}()
	intro := widget.NewLabel("Quality (hcenc) uses HCEnc BEST two-pass encoding. AviSynth+ and FFMS2 are included. easyDVD downloads HCEnc automatically when needed, verifies it and keeps a private copy for future jobs.")
	intro.Wrapping = fyne.TextWrapWord
	status := widget.NewLabel("")
	status.Wrapping = fyne.TextWrapWord
	bar := widget.NewProgressBarInfinite()
	bar.Hide()
	var d dialog.Dialog
	var download *widget.Button
	finish := func(path string, err error) {
		if ctx.Err() != nil {
			return
		}
		fyne.Do(func() {
			if ctx.Err() != nil {
				return
			}
			bar.Stop()
			bar.Hide()
			download.Enable()
			if err == nil {
				tools := u.engine.Tools()
				tools.HCenc = path
				err = tools.RequireProfile(model.Settings{EncodingProfile: model.ProfileQuality})
				if err == nil {
					err = u.engine.SetTools(tools)
				}
			}
			if err != nil {
				status.SetText("HCEnc setup could not finish. You can use Balanced (ffmpeg) encoding and retry from Settings → Repair dependencies.\n" + err.Error())
				return
			}
			status.SetText("HCEnc is ready. You can start the queue.")
			if automatic {
				d.Hide()
			}
		})
	}
	download = widget.NewButton("Download HCEnc 0.28", func() {
		download.Disable()
		bar.Show()
		bar.Start()
		status.SetText("Downloading HCEnc from hank315.nl…")
		go func() {
			setupCtx, stop := context.WithTimeout(ctx, 5*time.Minute)
			defer stop()
			path, err := media.InstallRecommendedHCenc(setupCtx, func(p media.DownloadProgress) {
				fyne.Do(func() {
					if ctx.Err() == nil {
						text := p.Stage
						if p.Total > 0 {
							text += fmt.Sprintf(" · %.0f%%", 100*float64(p.Downloaded)/float64(p.Total))
						}
						status.SetText(text)
					}
				})
			})
			finish(path, err)
		}()
	})
	note := widget.NewLabel("Downloads 2.2 MB directly from the author at hank315.nl. The site uses HTTP; both the archive and encoder must match the expected checksums before installation. No codecs or PATH changes are installed.")
	note.Wrapping = fyne.TextWrapWord
	cache := widget.NewButton("Open encoder folder", func() { u.action(func() error { return platform.OpenFolder(media.HCencCacheRoot()) }) })
	d = dialog.NewCustom("Quality (hcenc) setup", "Close / cancel", container.NewVBox(intro, download, note, status, bar, cache), u.win)
	u.qualityDialog = d
	d.SetOnClosed(func() { cancel(); u.qualityDialog = nil })
	d.Resize(fyne.NewSize(660, 370))
	d.Show()
	if automatic {
		download.OnTapped()
	}
}
