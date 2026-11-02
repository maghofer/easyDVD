package ui

import (
	"slices"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// Short, mutually exclusive choices stay visible instead of opening a menu.
type headerButtons struct {
	widget.BaseWidget
	content   *fyne.Container
	Options   []string
	Selected  string
	buttons   []*widget.Button
	onChanged func(string)
	disabled  bool
}

func newHeaderButtons(options []string, changed func(string)) *headerButtons {
	b := &headerButtons{content: container.NewHBox(), onChanged: changed}
	b.ExtendBaseWidget(b)
	b.SetOptions(options)
	return b
}

func (b *headerButtons) SetOptions(options []string) {
	if slices.Equal(b.Options, options) {
		return
	}
	b.Options = slices.Clone(options)
	b.content.Objects, b.buttons = nil, nil
	for _, value := range options {
		button := widget.NewButton(value, func() { b.SetSelected(value) })
		enabled(button, !b.disabled)
		b.buttons = append(b.buttons, button)
		b.content.Add(button)
	}
	b.highlight()
}

func (b *headerButtons) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(b.content)
}

func (b *headerButtons) highlight() {
	for i, button := range b.buttons {
		button.Importance = widget.MediumImportance
		if b.Options[i] == b.Selected {
			button.Importance = widget.HighImportance
		}
		button.Refresh()
	}
}

func (b *headerButtons) SetSelected(value string) {
	changed := b.Selected != value
	b.Selected = value
	b.highlight()
	if changed && value != "" && b.onChanged != nil {
		b.onChanged(value)
	}
}

func (b *headerButtons) Enable() {
	b.disabled = false
	for _, button := range b.buttons {
		button.Enable()
	}
}
func (b *headerButtons) Disable() {
	b.disabled = true
	for _, button := range b.buttons {
		button.Disable()
	}
}
func (b *headerButtons) Disabled() bool { return b.disabled }

// Use the full wordmark at a readable size while retaining normal button
// focus, hover, pressed and disabled behavior.
type formatLogoButton struct {
	widget.Button
	logo fyne.Resource
}

func newFormatLogoButton(data []byte, name string, tapped func()) *formatLogoButton {
	b := &formatLogoButton{logo: fyne.NewStaticResource(name, data)}
	b.OnTapped = tapped
	b.ExtendBaseWidget(b)
	return b
}

func (b *formatLogoButton) CreateRenderer() fyne.WidgetRenderer {
	image := canvas.NewImageFromResource(theme.NewThemedResource(b.logo))
	image.FillMode = canvas.ImageFillContain
	return &formatLogoRenderer{WidgetRenderer: b.Button.CreateRenderer(), button: b, logo: image}
}

type formatLogoRenderer struct {
	fyne.WidgetRenderer
	button *formatLogoButton
	logo   *canvas.Image
}

func (r *formatLogoRenderer) MinSize() fyne.Size { return fyne.NewSize(84, 46) }
func (r *formatLogoRenderer) Objects() []fyne.CanvasObject {
	return append(slices.Clone(r.WidgetRenderer.Objects()), r.logo)
}
func (r *formatLogoRenderer) Layout(size fyne.Size) {
	r.WidgetRenderer.Layout(size)
	r.logo.Move(fyne.NewPos(12, 8))
	r.logo.Resize(fyne.NewSize(size.Width-24, size.Height-16))
}
func (r *formatLogoRenderer) Refresh() {
	r.WidgetRenderer.Refresh()
	color := theme.ColorNameForeground
	if r.button.Importance == widget.HighImportance {
		color = theme.ColorNameForegroundOnPrimary
	}
	if r.button.Disabled() {
		color = theme.ColorNameDisabled
	}
	r.logo.Resource = theme.NewColoredResource(r.button.logo, color)
	r.logo.Refresh()
}

type outputPathLayout struct{ label *widget.Label }

func (l outputPathLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	return fyne.NewSize(objects[0].MinSize().Width+64, 28)
}

type sourceSummaryLayout struct{ summary *widget.Label }

func (l sourceSummaryLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	return fyne.NewSize(objects[0].MinSize().Width+fyne.MeasureText(l.summary.Text, 12, l.summary.TextStyle).Width+12, 16)
}
func (l sourceSummaryLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	width := min(size.Width/2, fyne.MeasureText(l.summary.Text, 12, l.summary.TextStyle).Width+1)
	objects[0].Move(fyne.NewPos(0, 0))
	objects[0].Resize(fyne.NewSize(max(0, size.Width-width-12), size.Height))
	objects[1].Move(fyne.NewPos(size.Width-width, 0))
	objects[1].Resize(fyne.NewSize(width, size.Height))
}

type headerControlsTheme struct{ headerTextTheme }

func (h headerControlsTheme) Size(name fyne.ThemeSizeName) float32 {
	if name == theme.SizeNameInnerPadding {
		return 6
	}
	return h.headerTextTheme.Size(name)
}
func (l outputPathLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	width := min(fyne.MeasureText(l.label.Text, 12, l.label.TextStyle).Width+1, max(0, size.Width-64))
	height := objects[0].MinSize().Height
	objects[0].Move(fyne.NewPos(0, (size.Height-height)/2))
	objects[0].Resize(fyne.NewSize(width, height))
	for i, object := range objects[1:] {
		object.Move(fyne.NewPos(width+4+float32(i)*30, (size.Height-28)/2))
		object.Resize(fyne.NewSize(28, 28))
	}
}
