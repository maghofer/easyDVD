package ui

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"easydvd/internal/menu"
	"easydvd/internal/model"
	"easydvd/internal/packing"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"
)

func (u *window) menuSettings(id string) fyne.Window {
	base := u.snapshot.Settings
	job := model.Job{Title: "DVD collection", Settings: base}
	if id != "" {
		found := false
		for _, j := range u.snapshot.Jobs {
			if j.ID == id {
				job = j.Clone()
				base = j.Settings
				found = true
				break
			}
		}
		if !found {
			return nil
		}
	}
	if len(job.Videos) == 0 {
		for i := 1; i <= 14; i++ {
			job.Videos = append(job.Videos, model.Video{ID: fmt.Sprint(i), Label: fmt.Sprintf("Family film %03d", i), Duration: 60})
		}
	}
	w := u.app.NewWindow("Menu settings — " + job.Title)
	w.Resize(fyne.NewSize(1140, 760))
	ctx, cancel := context.WithCancel(context.Background())
	if len(job.Discs) == 0 && id != "" {
		var items []packing.Item
		for i, v := range job.Videos {
			if !v.Excluded && v.Error == "" {
				items = append(items, packing.Item{ID: v.ID, Bytes: v.EstimatedBytes(), Order: i})
			}
		}
		if plan, err := packing.Plan(ctx, items, base.Capacity(), base.Packing == "Space efficiency", 0); err == nil {
			job.Discs = plan.Discs
		}
	}
	var worker sync.Mutex
	session := filepath.Join(base.WorkDir, "previews", "menu-settings-"+model.ID())
	w.SetOnClosed(func() { cancel(); go func() { worker.Lock(); defer worker.Unlock(); _ = os.RemoveAll(session) }() })
	style := base.Menu.Defaults()
	muted := false
	aspect := widget.NewSelect([]string{"4:3", "16:9"}, nil)
	menuLayout := widget.NewSelect([]string{"Text", "Thumbnails"}, nil)
	density := widget.NewEntry()
	fontSize := widget.NewEntry()
	headingSize := widget.NewEntry()
	spacing := widget.NewEntry()
	fontChoice := widget.NewSelect([]string{"Sans", "Monospace", "Custom"}, nil)
	fontLabel := widget.NewLabel("")
	fontLabel.Truncation = fyne.TextTruncateEllipsis
	backgroundLabel := widget.NewLabel("")
	backgroundLabel.Truncation = fyne.TextTruncateEllipsis
	fontColor := widget.NewEntry()
	highlightColor := widget.NewEntry()
	dimming := widget.NewSlider(0, 90)
	dimming.Step = 5
	dimLabel := widget.NewLabel("")
	picture := canvas.NewImageFromImage(nil)
	picture.FillMode = canvas.ImageFillContain
	picture.SetMinSize(fyne.NewSize(560, 400))
	status := widget.NewLabel("Preparing preview…")
	status.Wrapping = fyne.TextWrapWord
	scope := widget.NewLabel("Changes apply to this pending job. Presets are available for any new set.")
	if id == "" {
		scope.SetText("Defaults apply to new and pending jobs.")
	} else if job.Locked {
		scope.SetText("Started/finished set: preview changes and save presets here. Apply defaults for new jobs; existing ISOs stay unchanged.")
	}
	scope.Wrapping = fyne.TextWrapWord
	discChoice := widget.NewSelect(nil, nil)
	pageChoice := widget.NewSelect(nil, nil)
	discOptions := []string{"1"}
	if len(job.Discs) > 0 {
		discOptions = nil
		for i := range job.Discs {
			discOptions = append(discOptions, strconv.Itoa(i+1))
		}
	}
	discChoice.SetOptions(discOptions)
	discChoice.SetSelectedIndex(0)
	pageChoice.SetOptions([]string{"1"})
	pageChoice.SetSelectedIndex(0)
	var sequence atomic.Int64
	var timer *time.Timer
	var stopRender context.CancelFunc
	thumbs := map[string]string{}
	read := func() (model.Settings, error) {
		s := base
		s.Menu = style
		s.Menu.Aspect = aspect.Selected
		s.Menu.Font = fontChoice.Selected
		s.Menu.FontColor = strings.TrimSpace(fontColor.Text)
		s.Menu.HighlightColor = strings.TrimSpace(highlightColor.Text)
		fields := []struct {
			name, text string
			out        *int
		}{{"Font size", fontSize.Text, &s.Menu.FontSize}, {"Heading size", headingSize.Text, &s.Menu.HeadingSize}, {"Line spacing", spacing.Text, &s.Menu.LineSpacing}, {"Titles per page", density.Text, &s.TitlesPerPage}}
		for _, f := range fields {
			n, e := strconv.Atoi(strings.TrimSpace(f.text))
			if e != nil {
				return s, fmt.Errorf("%s must be a whole number", f.name)
			}
			*f.out = n
		}
		s.Menu.Dimming = int(dimming.Value)
		s.Layout = menuLayout.Selected
		return s, s.Validate()
	}
	var request func()
	request = func() {
		if muted || ctx.Err() != nil {
			return
		}
		if timer != nil {
			timer.Stop()
		}
		if stopRender != nil {
			stopRender()
		}
		seq := sequence.Add(1)
		s, err := read()
		if err != nil {
			status.SetText(err.Error())
			return
		}
		disc, _ := strconv.Atoi(discChoice.Selected)
		if disc < 1 {
			disc = 1
		}
		var videos []model.Video
		if len(job.Discs) > 0 {
			for _, vid := range job.Discs[disc-1].VideoIDs {
				for _, v := range job.Videos {
					if v.ID == vid {
						videos = append(videos, v)
						break
					}
				}
			}
		} else {
			for _, v := range job.Videos {
				if !v.Excluded && v.Error == "" {
					videos = append(videos, v)
				}
			}
		}
		if len(videos) == 0 {
			status.SetText("No included titles to preview.")
			return
		}
		pageCount := (len(videos) + s.TitlesPerPage - 1) / s.TitlesPerPage
		page, _ := strconv.Atoi(pageChoice.Selected)
		if page < 1 || page > pageCount {
			page = 1
		}
		opts := make([]string, pageCount)
		for i := range opts {
			opts[i] = strconv.Itoa(i + 1)
		}
		muted = true
		pageChoice.SetOptions(opts)
		pageChoice.SetSelected(strconv.Itoa(page))
		muted = false
		dimLabel.SetText(fmt.Sprintf("%d%%", int(dimming.Value)))
		renderCtx, stop := context.WithCancel(ctx)
		stopRender = stop
		status.SetText("Rendering preview…")
		timer = time.AfterFunc(150*time.Millisecond, func() {
			worker.Lock()
			defer worker.Unlock()
			if renderCtx.Err() != nil {
				return
			}
			began := time.Now()
			if s.Layout == "Thumbnails" {
				for i := (page - 1) * s.TitlesPerPage; i < min(page*s.TitlesPerPage, len(videos)); i++ {
					v := &videos[i]
					if renderCtx.Err() != nil {
						return
					}
					if cached := thumbs[v.ID]; cached != "" {
						v.Thumbnail = cached
						continue
					}
					if _, e := os.Stat(v.Thumbnail); e == nil {
						continue
					}
					if v.Path == "" {
						continue
					}
					if e := os.MkdirAll(session, 0755); e != nil {
						err = e
						break
					}
					v.Thumbnail = filepath.Join(session, model.Hash(v.ID)+".jpg")
					if e := u.engine.Tools().Thumbnail(renderCtx, *v, v.Path, v.Thumbnail, nil); e != nil {
						err = e
						break
					}
					thumbs[v.ID] = v.Thumbnail
				}
			}
			if renderCtx.Err() != nil {
				return
			}
			var img image.Image
			if err == nil {
				img, err = menu.Preview(job.Title, disc, len(discOptions), videos, s, page)
			}
			elapsed := time.Since(began)
			fyne.Do(func() {
				if ctx.Err() != nil || sequence.Load() != seq {
					return
				}
				if err != nil {
					status.SetText(err.Error())
					return
				}
				picture.Image = img
				picture.File = ""
				picture.Resource = nil
				picture.Refresh()
				status.SetText(fmt.Sprintf("Preview rendered in %d ms · text fits automatically within TV-safe margins.", elapsed.Milliseconds()))
			})
		})
	}
	fill := func(s model.Settings) {
		muted = true
		style = s.Menu.Defaults()
		aspect.SetSelected(style.Aspect)
		menuLayout.SetSelected(s.Layout)
		density.SetText(strconv.Itoa(s.TitlesPerPage))
		fontChoice.SetSelected(style.Font)
		fontSize.SetText(strconv.Itoa(style.FontSize))
		headingSize.SetText(strconv.Itoa(style.HeadingSize))
		spacing.SetText(strconv.Itoa(style.LineSpacing))
		fontColor.SetText(style.FontColor)
		highlightColor.SetText(style.HighlightColor)
		dimming.SetValue(float64(style.Dimming))
		name := style.BackgroundName
		if name == "" {
			name = filepath.Base(style.Background)
		}
		if style.Background == "" {
			name = "Generic background"
		}
		backgroundLabel.SetText(name)
		name = style.FontName
		if name == "" {
			name = filepath.Base(style.FontFile)
		}
		if style.FontFile == "" {
			name = "Built-in fonts included"
		}
		fontLabel.SetText(name)
		muted = false
		request()
	}
	choose := func(kind string) {
		picker := dialog.NewFileOpen(func(file fyne.URIReadCloser, err error) {
			if err != nil {
				dialog.ShowError(err, w)
				return
			}
			if file == nil {
				return
			}
			path := file.URI().Path()
			name := filepath.Base(path)
			file.Close()
			status.SetText("Importing " + kind + "…")
			go func() {
				path, e := u.engine.ImportMenuAsset(ctx, path, kind)
				fyne.Do(func() {
					if ctx.Err() != nil {
						return
					}
					if e != nil {
						dialog.ShowError(e, w)
						return
					}
					if kind == "background" {
						style.Background = path
						style.BackgroundName = name
						backgroundLabel.SetText(style.BackgroundName)
					} else {
						style.FontFile = path
						style.FontName = name
						fontLabel.SetText(style.FontName)
						fontChoice.SetSelected("Custom")
					}
					request()
				})
			}()
		}, w)
		extensions := []string{".png", ".jpg", ".jpeg"}
		if kind == "font" {
			extensions = []string{".ttf", ".otf"}
		}
		picker.SetFilter(storage.NewExtensionFileFilter(extensions))
		picker.Resize(fyne.NewSize(860, 580))
		picker.Show()
	}
	colorField := func(entry *widget.Entry, title string) fyne.CanvasObject {
		return container.NewBorder(nil, nil, nil, widget.NewButton("Choose…", func() {
			d := dialog.NewColorPicker(title, "", func(c color.Color) {
				r, g, b, _ := c.RGBA()
				entry.SetText(fmt.Sprintf("#%02X%02X%02X", r>>8, g>>8, b>>8))
			}, w)
			d.Show()
		}), entry)
	}
	var presets []model.MenuPreset
	preset := widget.NewSelect([]string{"Current settings", "Default"}, nil)
	preset.SetSelected("Current settings")
	var refreshPresets func()
	refreshPresets = func() {
		go func() {
			list, e := u.engine.MenuPresets()
			fyne.Do(func() {
				if ctx.Err() != nil {
					return
				}
				if e != nil {
					status.SetText(e.Error())
					return
				}
				presets = list
				names := []string{"Current settings", "Default"}
				for _, p := range presets {
					names = append(names, p.Name)
				}
				preset.SetOptions(names)
			})
		}()
	}
	preset.OnChanged = func(name string) {
		if name == "Current settings" {
			fill(base)
		} else if name == "Default" {
			s := base
			s.Menu = model.DefaultMenuStyle()
			s.Layout = "Text"
			s.TitlesPerPage = 10
			fill(s)
		} else {
			for _, p := range presets {
				if p.Name == name {
					s := base
					s.Menu = p.Style
					s.Layout = p.Layout
					s.TitlesPerPage = p.TitlesPerPage
					fill(s)
					break
				}
			}
		}
	}
	savePreset := widget.NewButton("Save preset…", func() {
		s, e := read()
		if e != nil {
			dialog.ShowError(e, w)
			return
		}
		name := widget.NewEntry()
		name.SetPlaceHolder("Preset name")
		d := dialog.NewForm("Save menu preset", "Save", "Cancel", []*widget.FormItem{widget.NewFormItem("Name", name)}, func(ok bool) {
			if !ok {
				return
			}
			p := model.MenuPreset{Name: strings.TrimSpace(name.Text), Style: s.Menu, Layout: s.Layout, TitlesPerPage: s.TitlesPerPage}
			if strings.EqualFold(p.Name, "Default") || strings.EqualFold(p.Name, "Current settings") {
				dialog.ShowError(fmt.Errorf("choose a different preset name"), w)
				return
			}
			go func() {
				e := u.engine.SaveMenuPreset(p)
				fyne.Do(func() {
					if ctx.Err() != nil {
						return
					}
					if e != nil {
						dialog.ShowError(e, w)
						return
					}
					refreshPresets()
					status.SetText("Preset saved: " + p.Name)
				})
			}()
		}, w)
		d.Show()
	})
	apply := func(target string) {
		s, e := read()
		if e != nil {
			dialog.ShowError(e, w)
			return
		}
		go func() {
			e := u.engine.SetMenu(target, s.Menu, s.Layout, s.TitlesPerPage)
			fyne.Do(func() {
				if ctx.Err() != nil {
					return
				}
				if e != nil {
					dialog.ShowError(e, w)
					return
				}
				status.SetText("Menu settings saved.")
			})
		}()
	}
	applyJob := widget.NewButton("Apply to job", func() { apply(id) })
	if id == "" || job.Locked {
		applyJob.Disable()
	}
	defaults := widget.NewButton("Save as defaults", func() { apply("") })
	for _, e := range []*widget.Entry{density, fontSize, headingSize, spacing, fontColor, highlightColor} {
		e.OnChanged = func(string) { request() }
	}
	aspect.OnChanged = func(string) { request() }
	fontChoice.OnChanged = func(string) { request() }
	menuLayout.OnChanged = func(v string) {
		if v == "Thumbnails" {
			if n, _ := strconv.Atoi(density.Text); n > 6 {
				density.SetText("6")
			}
		}
		request()
	}
	dimming.OnChanged = func(float64) { request() }
	discChoice.OnChanged = func(string) { request() }
	pageChoice.OnChanged = func(string) { request() }
	bgButtons := container.NewHBox(widget.NewButton("Load image…", func() { choose("background") }), widget.NewButton("Clear", func() {
		style.Background = ""
		style.BackgroundName = ""
		backgroundLabel.SetText("Generic background")
		request()
	}))
	form := widget.NewForm(widget.NewFormItem("Menu aspect", aspect), widget.NewFormItem("Layout", menuLayout), widget.NewFormItem("Titles per page", density), widget.NewFormItem("Background", container.NewVBox(backgroundLabel, bgButtons)), widget.NewFormItem("Dim image", container.NewVBox(dimming, dimLabel)), widget.NewFormItem("Font", container.NewVBox(fontChoice, fontLabel, widget.NewButton("Load font (TTF/OTF)…", func() { choose("font") }))), widget.NewFormItem("Font color", colorField(fontColor, "Font color")), widget.NewFormItem("Label size (px)", fontSize), widget.NewFormItem("Heading size (px)", headingSize), widget.NewFormItem("Line gap (px)", spacing), widget.NewFormItem("Highlight", colorField(highlightColor, "Selection highlight")))
	controls := container.NewBorder(container.NewVBox(scope, container.NewBorder(nil, nil, nil, savePreset, preset)), nil, nil, nil, container.NewVScroll(form))
	preview := container.NewBorder(container.NewHBox(widget.NewLabel("DVD"), discChoice, widget.NewLabel("Page"), pageChoice), status, nil, nil, picture)
	split := container.NewHSplit(controls, preview)
	split.Offset = .43
	buttons := container.NewHBox(layout.NewSpacer(), widget.NewButton("Close", w.Close), defaults, applyJob)
	w.SetContent(container.NewPadded(container.NewBorder(nil, buttons, nil, nil, split)))
	fill(base)
	refreshPresets()
	w.Show()
	return w
}
