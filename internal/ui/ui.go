package ui

import (
	"context"
	"fmt"
	"image/color"
	"path/filepath"
	"strings"
	"time"

	"easydvd"
	"easydvd/internal/app"
	"easydvd/internal/engine"
	"easydvd/internal/media"
	"easydvd/internal/model"
	"easydvd/internal/platform"
	"easydvd/internal/store"

	"fyne.io/fyne/v2"
	fyneapp "fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type professionalTheme struct{}

func (professionalTheme) Color(name fyne.ThemeColorName, v fyne.ThemeVariant) color.Color {
	if name == theme.ColorNamePrimary {
		return color.NRGBA{R: 83, G: 138, B: 198, A: 255}
	}
	return theme.DefaultTheme().Color(name, v)
}
func (professionalTheme) Font(style fyne.TextStyle) fyne.Resource {
	return theme.DefaultTheme().Font(style)
}
func (professionalTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}
func (professionalTheme) Size(name fyne.ThemeSizeName) float32 {
	if name == theme.SizeNamePadding {
		return 8
	}
	return theme.DefaultTheme().Size(name)
}

type window struct {
	mediaReady                                              bool
	mediaChecking                                           bool
	mediaIssue                                              string
	mediaDialog                                             dialog.Dialog
	dropBar                                                 *fyne.Container
	app                                                     fyne.App
	win                                                     fyne.Window
	engine                                                  *engine.Engine
	snapshot                                                engine.Snapshot
	selected                                                string
	selectionJob                                            string
	selectionAnchor                                         string
	selectedFiles                                           map[string]bool
	selectedJobs                                            map[string]bool
	jobSelectionAnchor                                      string
	jobSelectAll                                            *widget.Button
	jobSelectionCount                                       *widget.Label
	fileHints                                               *fyne.Container
	selectAll                                               *widget.Button
	selectionCount                                          *widget.Label
	excludeSelected, includeSelected                        *widget.Button
	queue                                                   *jobList
	files                                                   *fileList
	title, status, source                                   *widget.Label
	queueProgress                                           *thinProgress
	queuePosition, queuePercent                             *widget.Label
	footerProgress                                          *fyne.Container
	jobHeader                                               *fyne.Container
	jobSummary, jobOutput                                   *widget.Label
	jobWarning, clear                                       *widget.Button
	renameJob                                               *fileAction
	details                                                 *fyne.Container
	empty                                                   *fyne.Container
	start, pause, cancel, retry, settings, remove, up, down *widget.Button
	closed                                                  chan struct{}
}

func ShowStartupError(err error) {
	a := fyneapp.NewWithID("org.easydvd.startup")
	a.SetIcon(applicationIcon())
	w := a.NewWindow("easyDVD — Unable to start")
	label := widget.NewLabel(err.Error())
	label.Wrapping = fyne.TextWrapWord
	w.SetContent(container.NewPadded(container.NewBorder(nil, widget.NewButton("Close", func() { w.Close() }), nil, nil, label)))
	w.Resize(fyne.NewSize(620, 220))
	w.ShowAndRun()
}

func Run() error {
	config, base := app.Paths()
	st, err := store.Open(filepath.Join(config, "queue.db"))
	if err != nil {
		return err
	}
	defer st.Close()
	dataDir, err := app.LocalDataDir()
	if err != nil {
		return err
	}
	e, err := engine.New(st, model.DefaultSettings(base), media.FindAuthoringTools(), engine.WithDataDir(dataDir))
	if err != nil {
		return err
	}
	defer e.Close()
	a := fyneapp.NewWithID("org.easydvd.desktop")
	a.SetIcon(applicationIcon())
	a.Settings().SetTheme(professionalTheme{})
	w := a.NewWindow("easyDVD " + easydvd.Version())
	u := &window{app: a, win: w, engine: e, snapshot: e.Snapshot(), closed: make(chan struct{})}
	u.applyTheme(u.snapshot.Settings.Theme)
	w.Resize(fyne.NewSize(1160, 780))
	w.SetContent(u.build())
	w.SetMaster()
	w.SetOnDropped(func(_ fyne.Position, uris []fyne.URI) {
		if !u.ensureMedia() {
			return
		}
		var paths []string
		for _, uri := range uris {
			if uri.Scheme() != "file" {
				continue
			}
			p := uri.Path()
			if len(p) > 2 && p[0] == '/' && p[2] == ':' {
				p = p[1:]
			}
			p = filepath.FromSlash(p)
			paths = append(paths, p)
		}
		u.addDroppedPaths(paths)
	})
	w.SetCloseIntercept(func() {
		s := u.snapshot
		active := false
		for _, j := range s.Jobs {
			if j.State == model.Running || j.State == model.Scanning {
				active = true
				break
			}
		}
		if !active {
			w.Close()
			return
		}
		dialog.ShowConfirm("Close easyDVD?", "Active stages will stop. Completed work is saved and can be resumed next time.", func(ok bool) {
			if ok {
				w.Close()
			}
		}, w)
	})
	w.SetOnClosed(func() { close(u.closed) })
	u.refresh()
	go u.watch()
	u.findMediaTools()
	w.ShowAndRun()
	return nil
}

func (u *window) action(fn func() error) {
	go func() {
		if err := fn(); err != nil {
			fyne.Do(func() { dialog.ShowError(err, u.win) })
		}
	}()
}
func (u *window) selectedJob() *model.Job {
	for i := range u.snapshot.Jobs {
		if u.snapshot.Jobs[i].ID == u.selected {
			return &u.snapshot.Jobs[i]
		}
	}
	return nil
}
func (u *window) applyTheme(s string) {
	switch s {
	case "Dark":
		u.app.Settings().SetTheme(&variantTheme{theme.VariantDark})
	case "Light":
		u.app.Settings().SetTheme(&variantTheme{theme.VariantLight})
	default:
		u.app.Settings().SetTheme(professionalTheme{})
	}
}

type variantTheme struct{ variant fyne.ThemeVariant }

func (v *variantTheme) Color(n fyne.ThemeColorName, _ fyne.ThemeVariant) color.Color {
	return professionalTheme{}.Color(n, v.variant)
}
func (v *variantTheme) Font(s fyne.TextStyle) fyne.Resource     { return professionalTheme{}.Font(s) }
func (v *variantTheme) Icon(n fyne.ThemeIconName) fyne.Resource { return professionalTheme{}.Icon(n) }
func (v *variantTheme) Size(n fyne.ThemeSizeName) float32       { return professionalTheme{}.Size(n) }

func (u *window) addFolders() {
	if !u.ensureMedia() {
		return
	}
	u.action(func() error {
		paths, err := platform.PickFolders(true)
		if err != nil {
			return err
		}
		for _, p := range paths {
			if err = u.engine.Add(p); err != nil {
				return err
			}
		}
		return nil
	})
}

func (u *window) build() fyne.CanvasObject {
	brand := newBrandLogo()
	u.start = widget.NewButtonWithIcon("Start queue", theme.MediaPlayIcon(), func() {
		if u.ensureMedia() {
			u.action(func() error { u.engine.Start(); return nil })
		}
	})
	u.start.Importance = widget.HighImportance
	u.pause = widget.NewButtonWithIcon("Pause", theme.MediaPauseIcon(), func() { u.action(func() error { u.engine.Pause(); return nil }) })
	preferences := widget.NewButtonWithIcon("Default settings", theme.SettingsIcon(), func() { u.settingsDialog("", u.snapshot.Settings) })
	diagnostics := newFileAction(u, nil, "Media tools")
	diagnostics.SetText("!")
	diagnostics.Importance = widget.MediumImportance
	diagnostics.OnTapped = u.diagnostics
	controls := container.NewCenter(container.NewHBox(u.start, u.pause, preferences, diagnostics))
	top := container.NewVBox(container.NewBorder(nil, nil, brand, controls), widget.NewSeparator())
	queue := u.buildJobList()
	dropLabel := widget.NewLabelWithStyle("Drop folders or video files here", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	sub := widget.NewLabelWithStyle("Folders include subfolders · Group files into DVD sets", fyne.TextAlignCenter, fyne.TextStyle{})
	border := canvas.NewRectangle(color.Transparent)
	border.StrokeColor = theme.DisabledColor()
	border.StrokeWidth = 1
	border.CornerRadius = 5
	drop := container.NewStack(border, container.NewPadded(container.NewVBox(dropLabel, sub)))
	fileList := u.buildFileList()
	detailsButton := widget.NewButtonWithIcon("Details", theme.InfoIcon(), u.jobDetails)
	u.cancel = widget.NewButton("Cancel job", func() { id := u.selected; u.action(func() error { u.engine.Cancel(id); return nil }) })
	u.retry = widget.NewButtonWithIcon("Resume / retry", theme.ViewRefreshIcon(), func() { id := u.selected; u.action(func() error { u.engine.Retry(id); return nil }) })
	detailTop := u.buildJobHeader()
	detailBottom := container.NewHBox(u.cancel, u.retry, layout.NewSpacer(), detailsButton)
	u.details = container.NewBorder(detailTop, detailBottom, nil, nil, fileList)
	largeBorder := canvas.NewRectangle(color.Transparent)
	largeBorder.StrokeColor = theme.DisabledColor()
	largeBorder.StrokeWidth = 1
	largeBorder.CornerRadius = 5
	largeDrop := container.NewStack(largeBorder, container.NewPadded(container.NewVBox(layout.NewSpacer(), widget.NewLabelWithStyle("Drop folders or video files here", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}), widget.NewLabelWithStyle("Folders include subfolders · Group files into DVD sets", fyne.TextAlignCenter, fyne.TextStyle{}), container.NewCenter(widget.NewButtonWithIcon("Add folders", theme.FolderOpenIcon(), u.addFolders)), layout.NewSpacer())))
	u.empty = container.NewCenter(container.NewGridWrap(fyne.NewSize(510, 230), largeDrop))
	u.dropBar = container.NewVBox(drop, widget.NewSeparator())
	right := container.NewBorder(u.dropBar, nil, nil, nil, container.NewStack(u.empty, u.details))
	split := container.NewHSplit(queue, right)
	split.Offset = .27
	u.status = widget.NewLabel("Ready")
	u.status.Truncation = fyne.TextTruncateEllipsis
	content := container.NewPadded(container.NewBorder(top, u.buildStatusBar(), nil, nil, split))
	u.fileHints = container.NewWithoutLayout()
	return container.NewStack(content, u.fileHints)
}

func (u *window) watch() {
	tick := time.NewTicker(300 * time.Millisecond)
	defer tick.Stop()
	dirty := true
	for {
		select {
		case <-u.closed:
			return
		case <-u.engine.Events:
			dirty = true
		case <-tick.C:
			if dirty {
				s := u.engine.Snapshot()
				dirty = false
				fyne.Do(func() { u.snapshot = s; u.refresh() })
			}
		}
	}
}
func enabled(w fyne.Disableable, on bool) {
	if on {
		w.Enable()
	} else {
		w.Disable()
	}
}
func (u *window) refresh() {
	j := u.selectedJob()
	if j == nil && len(u.snapshot.Jobs) > 0 {
		u.selected = u.snapshot.Jobs[0].ID
		j = u.selectedJob()
	}
	enabled(u.pause, u.snapshot.Enabled)
	enabled(u.start, u.mediaReady && len(u.snapshot.Jobs) > 0 && !u.snapshot.Clearing)
	enabled(u.clear, len(u.snapshot.Jobs) > 0 && !u.snapshot.Clearing)
	if u.snapshot.Clearing {
		u.clear.SetText("Clearing…")
	} else {
		u.clear.SetText("Clear queue")
	}
	if u.snapshot.Enabled {
		u.start.SetText("Queue running")
		u.start.Disable()
	} else {
		u.start.SetText("Start / resume")
	}
	enabled(u.up, j != nil && !u.snapshot.Clearing)
	enabled(u.down, j != nil && !u.snapshot.Clearing)
	if j == nil {
		u.selected = ""
		u.details.Hide()
		u.empty.Show()
		u.dropBar.Hide()
	} else {
		u.empty.Hide()
		u.details.Show()
		u.dropBar.Show()
		enabled(u.settings, !j.Locked && !u.snapshot.Clearing)
		enabled(u.cancel, j.State != model.Complete && j.State != model.Canceled)
		enabled(u.retry, u.mediaReady && !u.snapshot.Clearing && !u.snapshot.Busy[j.ID] && (j.State == model.Blocked || j.State == model.Paused || j.State == model.Canceled))
	}
	u.refreshJobHeader(j)
	u.refreshFileSelection()
	u.files.Refresh()
	u.refreshJobSelection()
	u.queue.Refresh()
	counts := map[model.State]int{}
	failed, attention := 0, 0
	for _, j := range u.snapshot.Jobs {
		counts[j.State]++
		state, _ := jobAppearance(j)
		if state == "Failed" {
			failed++
		} else if state == "Needs attention" {
			attention++
		}
	}
	status := fmt.Sprintf("%d jobs · %d completed · %d failed · %d need attention", len(u.snapshot.Jobs), counts[model.Complete], failed, attention)
	if u.snapshot.Error != "" {
		status = u.snapshot.Error
	}
	if u.snapshot.Clearing {
		status = "Stopping active work and removing jobs…"
	}
	if u.mediaChecking {
		status = "Checking FFmpeg / FFprobe…"
	} else if !u.mediaReady {
		status = "FFmpeg setup required — use ! to download or locate a compatible installation"
	}
	u.status.SetText(status)
	u.refreshStatusProgress()
}

func (u *window) rename() {
	j := u.selectedJob()
	if j == nil {
		return
	}
	id := j.ID
	entry := widget.NewEntry()
	entry.SetText(j.Title)
	dialog.ShowForm("Set title", "Save", "Cancel", []*widget.FormItem{widget.NewFormItem("Title", entry)}, func(ok bool) {
		if ok {
			value := strings.TrimSpace(entry.Text)
			if value == "" {
				dialog.ShowError(fmt.Errorf("title cannot be empty"), u.win)
				return
			}
			u.action(func() error { return u.engine.Edit(id, value, "", "", nil) })
		}
	}, u.win)
}
func (u *window) diagnostics() {
	holder := container.NewStack(widget.NewLabel("Checking bundled tools…"))
	var d *dialog.CustomDialog
	setup := widget.NewButton("FFmpeg setup", func() { d.Hide(); u.showMediaSetup() })
	d = dialog.NewCustom("Media tools", "Close", container.NewBorder(nil, setup, nil, nil, holder), u.win)
	d.Resize(fyne.NewSize(760, 500))
	d.Show()
	u.action(func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		tools := u.engine.Tools()
		statuses := tools.Check(ctx)
		var text strings.Builder
		for _, s := range statuses {
			fmt.Fprintf(&text, "%s\n%s\n%s\n", s.Name, s.Path, s.Version)
			if s.Error != "" {
				fmt.Fprintf(&text, "%s\n", s.Error)
			}
			text.WriteString("\n")
		}
		fyne.Do(func() {
			_, view := newReadOnlyText(text.String())
			holder.Objects = []fyne.CanvasObject{container.NewScroll(view)}
			holder.Refresh()
		})
		return nil
	})
}
