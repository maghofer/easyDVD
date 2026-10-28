package ui

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"easydvd"
	"easydvd/internal/media"
	"easydvd/internal/platform"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func (u *window) ensureMedia() bool {
	if u.mediaReady {
		return true
	}
	if u.mediaChecking {
		dialog.ShowInformation("Checking media tools", "Checking existing FFmpeg and FFprobe installations. This runs in the background.", u.win)
	} else {
		u.showMediaSetup()
	}
	return false
}
func (u *window) findMediaTools() {
	u.mediaChecking = true
	u.refresh()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		go func() {
			select {
			case <-u.closed:
				cancel()
			case <-ctx.Done():
			}
		}()
		tools, err := media.ResolveTools(ctx, "")
		if err == nil {
			err = u.engine.SetTools(tools)
		}
		fyne.Do(func() {
			select {
			case <-u.closed:
				return
			default:
			}
			u.mediaChecking = false
			if err != nil {
				u.mediaIssue = err.Error()
				u.mediaReady = false
				u.refresh()
				u.showMediaSetup()
				return
			}
			u.mediaReady = true
			u.mediaIssue = ""
			u.refresh()
		})
	}()
}

func (u *window) showMediaSetup() {
	if u.mediaChecking {
		dialog.ShowInformation("Checking media tools", "Please wait for the background compatibility check to finish.", u.win)
		return
	}
	if u.mediaDialog != nil {
		return
	}
	if len(u.snapshot.Busy) > 0 {
		dialog.ShowInformation("FFmpeg is in use", "Wait for scanning and processing to stop before changing the FFmpeg installation.", u.win)
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
	pinned := easydvd.RecommendedFFmpeg()
	message := "The DVD authoring tools are included. FFmpeg and FFprobe are installed separately.\n" + media.SupportedFFmpeg + " is supported; both programs and the required DVD capabilities are checked."
	intro := widget.NewLabel(message)
	intro.Wrapping = fyne.TextWrapWord
	status := widget.NewLabel(u.mediaIssue)
	status.Wrapping = fyne.TextWrapWord
	if len(status.Text) > 650 {
		status.SetText(status.Text[:650] + "…")
	}
	directory := widget.NewEntry()
	directory.SetPlaceHolder("Folder containing ffmpeg.exe and ffprobe.exe")
	bar := widget.NewProgressBar()
	bar.Hide()
	spinner := widget.NewProgressBarInfinite()
	spinner.Hide()
	var d dialog.Dialog
	var download, browse, use *widget.Button
	busy := false
	setBusy := func(active bool) {
		busy = active
		enabled(download, !active)
		enabled(browse, !active)
		enabled(directory, !active)
		enabled(use, !active && strings.TrimSpace(directory.Text) != "")
		if !active {
			spinner.Stop()
			spinner.Hide()
			bar.Hide()
		}
	}
	run := func(action func(context.Context) (media.FFmpegPair, error)) {
		setBusy(true)
		status.SetText("Checking FFmpeg…")
		spinner.Show()
		spinner.Start()
		go func() {
			pair, err := action(ctx)
			var saveErr error
			if err == nil && ctx.Err() == nil {
				tools := u.engine.Tools()
				tools.FFmpeg = pair.FFmpeg
				tools.FFprobe = pair.FFprobe
				tools.FFmpegIssue = ""
				err = u.engine.SetTools(tools)
				if err == nil {
					saveErr = media.SaveFFmpegFolder(filepath.Dir(pair.FFmpeg))
				}
			}
			if ctx.Err() != nil {
				return
			}
			fyne.Do(func() {
				if ctx.Err() != nil {
					return
				}
				setBusy(false)
				if err != nil {
					status.SetText(err.Error())
					return
				}
				u.mediaReady = true
				u.mediaIssue = ""
				u.refresh()
				d.Hide()
				if saveErr != nil {
					dialog.ShowError(fmt.Errorf("FFmpeg is ready for this session, but its location could not be saved: %w", saveErr), u.win)
				}
			})
		}()
	}
	browse = widget.NewButtonWithIcon("", theme.FolderOpenIcon(), func() {
		u.action(func() error {
			folders, err := platform.PickFolders(false)
			if err == nil && len(folders) > 0 {
				fyne.Do(func() {
					if ctx.Err() == nil {
						directory.SetText(folders[0])
					}
				})
			}
			return err
		})
	})
	use = widget.NewButton("Use this folder", func() {
		folder := strings.TrimSpace(directory.Text)
		run(func(parent context.Context) (media.FFmpegPair, error) {
			ctx, cancel := context.WithTimeout(parent, 30*time.Second)
			defer cancel()
			return media.ValidateFFmpegFolder(ctx, folder)
		})
	})
	use.Disable()
	directory.OnChanged = func(value string) { enabled(use, !busy && strings.TrimSpace(value) != "") }
	download = widget.NewButtonWithIcon("Download recommended "+pinned.Version, theme.DownloadIcon(), func() {
		run(func(ctx context.Context) (media.FFmpegPair, error) {
			return media.InstallRecommendedFFmpeg(ctx, func(p media.DownloadProgress) {
				fyne.Do(func() {
					if ctx.Err() != nil {
						return
					}
					text := p.Stage
					if p.Total > 0 {
						spinner.Stop()
						spinner.Hide()
						bar.Show()
						bar.SetValue(float64(p.Downloaded) / float64(p.Total))
						text += fmt.Sprintf(" · %.1f / %.1f MB", float64(p.Downloaded)/1e6, float64(p.Total)/1e6)
					} else {
						bar.Hide()
						spinner.Show()
						spinner.Start()
					}
					status.SetText(text)
				})
			})
		})
	})
	download.Importance = widget.HighImportance
	note := widget.NewLabel("Download is optional and starts only when you click Download. It is verified and stored in easyDVD’s private application cache. Your system PATH is unchanged.")
	note.Wrapping = fyne.TextWrapWord
	content := container.NewVBox(intro, widget.NewSeparator(), widget.NewLabel("Use an existing installation"), container.NewBorder(nil, nil, nil, browse, directory), container.NewHBox(layout.NewSpacer(), use), widget.NewSeparator(), download, note, status, spinner, bar, container.NewHBox(layout.NewSpacer(), widget.NewButton("Close / cancel", func() { cancel(); d.Hide() })))
	d = dialog.NewCustomWithoutButtons("FFmpeg setup", container.NewVScroll(content), u.win)
	u.mediaDialog = d
	d.SetOnClosed(func() { cancel(); u.mediaDialog = nil })
	d.Resize(fyne.NewSize(760, 580))
	d.Show()
}
