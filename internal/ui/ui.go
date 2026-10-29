package ui

import (
	"context"
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"strconv"
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
	mediaReady                                                       bool
	mediaChecking                                                    bool
	mediaIssue                                                       string
	mediaDialog                                                      dialog.Dialog
	dropBar                                                          *fyne.Container
	app                                                              fyne.App
	win                                                              fyne.Window
	engine                                                           *engine.Engine
	snapshot                                                         engine.Snapshot
	selected                                                         string
	selectionJob                                                     string
	selectionAnchor                                                  string
	selectedFiles                                                    map[string]bool
	fileHints                                                        *fyne.Container
	selectAll                                                        *widget.Check
	selectionCount                                                   *widget.Label
	excludeSelected, includeSelected                                 *widget.Button
	queue, files                                                     *widget.List
	title, meta, stage, status, source                               *widget.Label
	progress                                                         *widget.ProgressBar
	scanProgress                                                     *widget.ProgressBarInfinite
	headerJob                                                        string
	headerExpanded                                                   bool
	headerInfo, jobHeader                                            *fyne.Container
	jobSummary, jobState, activity                                   *widget.Label
	headerToggle, jobWarning, clear                                  *widget.Button
	details                                                          *fyne.Container
	empty                                                            *fyne.Container
	start, pause, cancel, retry, settings, preview, remove, up, down *widget.Button
	closed                                                           chan struct{}
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
	preferences := widget.NewButtonWithIcon("Settings", theme.SettingsIcon(), func() { u.settingsDialog("", u.snapshot.Settings) })
	diagnostics := widget.NewButtonWithIcon("Tools", theme.InfoIcon(), u.diagnostics)
	controls := container.NewCenter(container.NewHBox(u.start, u.pause, preferences, diagnostics))
	top := container.NewVBox(container.NewBorder(nil, nil, brand, controls), widget.NewSeparator())
	u.queue = widget.NewList(func() int { return len(u.snapshot.Jobs) }, func() fyne.CanvasObject {
		a := widget.NewLabelWithStyle("Folder", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
		a.Truncation = fyne.TextTruncateEllipsis
		b := widget.NewLabel("Ready")
		b.Truncation = fyne.TextTruncateEllipsis
		return container.NewVBox(a, b)
	}, func(i widget.ListItemID, o fyne.CanvasObject) {
		if i >= len(u.snapshot.Jobs) {
			return
		}
		j := u.snapshot.Jobs[i]
		c := o.(*fyne.Container)
		c.Objects[0].(*widget.Label).SetText(j.Title)
		c.Objects[1].(*widget.Label).SetText(fmt.Sprintf("%s · %d videos", j.State, len(j.Videos)))
	})
	u.queue.OnSelected = func(i widget.ListItemID) {
		if i < len(u.snapshot.Jobs) {
			u.selected = u.snapshot.Jobs[i].ID
			u.refresh()
		}
	}
	u.up = widget.NewButtonWithIcon("", theme.MoveUpIcon(), func() { id := u.selected; u.action(func() error { u.engine.Move(id, -1); return nil }) })
	u.down = widget.NewButtonWithIcon("", theme.MoveDownIcon(), func() { id := u.selected; u.action(func() error { u.engine.Move(id, 1); return nil }) })
	u.remove = widget.NewButtonWithIcon("Remove", theme.DeleteIcon(), func() { id := u.selected; u.action(func() error { return u.engine.Remove(id) }) })
	u.clear = widget.NewButtonWithIcon("Clear queue", theme.DeleteIcon(), u.clearQueue)
	queueHeading := container.NewBorder(nil, nil, nil, u.clear, widget.NewLabelWithStyle("Queue", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}))
	queue := container.NewBorder(container.NewVBox(queueHeading, widget.NewSeparator()), container.NewHBox(u.up, u.down, layout.NewSpacer(), u.remove), nil, nil, u.queue)
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
	content := container.NewPadded(container.NewBorder(top, container.NewVBox(widget.NewSeparator(), u.status), nil, nil, split))
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
	u.queue.Refresh()
	for index, job := range u.snapshot.Jobs {
		if job.ID == u.selected {
			u.queue.Select(index)
			break
		}
	}
	j := u.selectedJob()
	if j == nil && len(u.snapshot.Jobs) > 0 {
		u.selected = u.snapshot.Jobs[0].ID
		u.queue.Select(0)
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
	enabled(u.remove, j != nil && !u.snapshot.Clearing && j.State != model.Running && j.State != model.Scanning)
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
		enabled(u.preview, len(j.ActiveVideos()) > 0 && j.ScanComplete && !j.Settings.NoMenu)
		enabled(u.cancel, j.State != model.Complete && j.State != model.Canceled)
		enabled(u.retry, u.mediaReady && !u.snapshot.Clearing && !u.snapshot.Busy[j.ID] && (j.State == model.Blocked || j.State == model.Paused || j.State == model.Canceled))
	}
	u.refreshJobHeader(j)
	u.refreshFileSelection()
	u.files.Refresh()
	counts := map[model.State]int{}
	for _, j := range u.snapshot.Jobs {
		counts[j.State]++
	}
	status := fmt.Sprintf("%d sets · %d running · %d complete · %d need attention", len(u.snapshot.Jobs), counts[model.Running], counts[model.Complete], counts[model.Blocked])
	if u.snapshot.Error != "" {
		status = u.snapshot.Error
	}
	if u.snapshot.Clearing {
		status = "Stopping active work and clearing the queue…"
	}
	if u.mediaChecking {
		status = "Checking FFmpeg / FFprobe…"
	} else if !u.mediaReady {
		status = "FFmpeg setup required — use Tools to download or locate a compatible installation"
	}
	u.status.SetText(status)
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
func (u *window) settingsDialog(id string, s model.Settings) {
	standard := widget.NewSelect([]string{"PAL", "NTSC"}, nil)
	standard.SetSelected(s.Standard)
	medium := widget.NewSelect([]string{"DVD-5", "DVD-9"}, nil)
	medium.SetSelected(s.Media)
	packingMode := widget.NewSelect([]string{"Strict order", "Space efficiency"}, nil)
	packingMode.SetSelected(s.Packing)
	perPage := widget.NewEntry()
	perPage.SetText(strconv.Itoa(s.TitlesPerPage))
	menuLayout := widget.NewSelect([]string{"Text", "Thumbnails"}, func(v string) {
		n, _ := strconv.Atoi(perPage.Text)
		if v == "Thumbnails" && n > 6 {
			perPage.SetText("6")
		}
	})
	menuLayout.SetSelected(s.Layout)
	stage := widget.NewCheck("Stage sources locally (recommended for network shares)", nil)
	stage.SetChecked(s.StageLocally)
	keep := widget.NewCheck("Keep intermediate media after success", nil)
	keep.SetChecked(s.KeepWork)
	noMenu := widget.NewCheck("No menu — start playback automatically (one included video)", nil)
	noMenu.SetChecked(s.NoMenu)
	if !s.NoMenu && (id == "" || u.selectedJob() == nil || len(u.selectedJob().ActiveVideos()) != 1) {
		noMenu.Disable()
	}
	workers := widget.NewSelect([]string{"1", "2"}, nil)
	workers.SetSelected(strconv.Itoa(s.Encoders))
	output := widget.NewEntry()
	output.SetText(s.OutputDir)
	work := widget.NewEntry()
	work.SetText(s.WorkDir)
	browse := func(entry *widget.Entry) {
		u.action(func() error {
			paths, err := platform.PickFolders(false)
			if err == nil && len(paths) > 0 {
				fyne.Do(func() { entry.SetText(paths[0]) })
			}
			return err
		})
	}
	folder := func(entry *widget.Entry) fyne.CanvasObject {
		return container.NewBorder(nil, nil, nil, widget.NewButtonWithIcon("", theme.FolderOpenIcon(), func() { browse(entry) }), entry)
	}
	appearance := widget.NewSelect([]string{"System", "Light", "Dark"}, nil)
	appearance.SetSelected(s.Theme)
	form := widget.NewForm(widget.NewFormItem("TV standard", standard), widget.NewFormItem("Disc capacity", medium), widget.NewFormItem("File allocation", packingMode), widget.NewFormItem("Menu layout", menuLayout), widget.NewFormItem("Titles per page", perPage), widget.NewFormItem("Encoding workers", workers), widget.NewFormItem("Output folder", folder(output)), widget.NewFormItem("Local work folder", folder(work)), widget.NewFormItem("Appearance", appearance))
	form.Append("Playback", noMenu)
	note := "Applies to new and pending jobs. Started jobs retain their settings."
	if id != "" {
		note = "Settings for this folder. Locked when processing starts."
	}
	heading := widget.NewLabel(note + "\nQuality: two-pass MPEG-2, 9 Mbps target / peak, AC-3 448 kbps.")
	heading.Wrapping = fyne.TextWrapWord
	var d dialog.Dialog
	save := func() {
		n, err := strconv.Atoi(perPage.Text)
		if err != nil {
			dialog.ShowError(fmt.Errorf("titles per page must be a whole number"), u.win)
			return
		}
		next := s
		next.Standard = standard.Selected
		next.Media = medium.Selected
		next.Packing = packingMode.Selected
		next.Layout = menuLayout.Selected
		next.TitlesPerPage = n
		next.Encoders, _ = strconv.Atoi(workers.Selected)
		next.StageLocally = stage.Checked
		next.KeepWork = keep.Checked
		next.NoMenu = noMenu.Checked
		next.OutputDir = strings.TrimSpace(output.Text)
		next.WorkDir = strings.TrimSpace(work.Text)
		next.Theme = appearance.Selected
		if err = next.Validate(); err != nil {
			dialog.ShowError(err, u.win)
			return
		}
		u.action(func() error {
			err := u.engine.SetSettings(id, next)
			if err == nil {
				fyne.Do(func() {
					if id == "" {
						u.applyTheme(next.Theme)
					}
					d.Hide()
				})
			}
			return err
		})
	}
	buttons := container.NewHBox(layout.NewSpacer(), widget.NewButton("Cancel", func() { d.Hide() }), widget.NewButton("Save settings", save))
	if id == "" {
		buttons.Objects = append([]fyne.CanvasObject{widget.NewButton("Menu settings…", func() { d.Hide(); u.menuSettings("") })}, buttons.Objects...)
		openLogs := widget.NewButtonWithIcon("Open logs folder", theme.FolderOpenIcon(), func() {
			u.action(func() error {
				if err := os.MkdirAll(u.engine.LogsDir(), 0755); err != nil {
					return err
				}
				return platform.OpenFolder(u.engine.LogsDir())
			})
		})
		buttons.Objects = append([]fyne.CanvasObject{openLogs}, buttons.Objects...)
	}
	options := container.NewVBox(form, stage, keep)
	if id == "" {
		options.Add(u.updateSettings())
	}
	content := container.NewBorder(heading, buttons, nil, nil, container.NewVScroll(options))
	d = dialog.NewCustomWithoutButtons("Settings", content, u.win)
	d.Resize(fyne.NewSize(740, 600))
	d.Show()
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
			entry := widget.NewMultiLineEntry()
			entry.SetText(text.String())
			entry.Disable()
			holder.Objects = []fyne.CanvasObject{container.NewScroll(entry)}
			holder.Refresh()
		})
		return nil
	})
}
