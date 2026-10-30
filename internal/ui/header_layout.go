package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// The header keeps the former collapsed height regardless of paths or state.
type fixedHeaderLayout struct{}

func (fixedHeaderLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	width := float32(0)
	for _, object := range objects {
		width = max(width, object.MinSize().Width)
	}
	return fyne.NewSize(width, 80)
}

// Keep the edit icon beside the visible title instead of at the far edge of
// the header. Long titles truncate before reaching the icon.
type titleEditLayout struct{ title *widget.Label }

func (l titleEditLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	return fyne.NewSize(objects[0].MinSize().Width+32, 28)
}
func (l titleEditLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	title, edit := objects[0], objects[1]
	textWidth := fyne.MeasureText(l.title.Text, 14, l.title.TextStyle).Width + 1
	width := min(textWidth, max(0, size.Width-32))
	height := title.MinSize().Height
	title.Move(fyne.NewPos(0, (size.Height-height)/2))
	title.Resize(fyne.NewSize(width, height))
	edit.Move(fyne.NewPos(width+4, (size.Height-28)/2))
	edit.Resize(fyne.NewSize(28, 28))
}
func (fixedHeaderLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	for _, object := range objects {
		object.Resize(size)
	}
}

type tightHeaderLayout struct{}

func (tightHeaderLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	size := fyne.NewSize(0, 0)
	for i, object := range objects {
		child := object.MinSize()
		size.Width = max(size.Width, child.Width)
		size.Height += child.Height
		if i > 0 {
			size.Height += 2
		}
	}
	size.Width += 8
	return size
}
func (l tightHeaderLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	y := max(0, (size.Height-l.MinSize(objects).Height)/2)
	for _, object := range objects {
		height := object.MinSize().Height
		object.Move(fyne.NewPos(8, y))
		object.Resize(fyne.NewSize(max(0, size.Width-8), height))
		y += height + 2
	}
}

// Remove label padding only within the information column. Colors still
// follow the app theme, including a later light/dark appearance change.
type headerTextTheme struct{ size float32 }

func (headerTextTheme) Color(n fyne.ThemeColorName, v fyne.ThemeVariant) color.Color {
	return fyne.CurrentApp().Settings().Theme().Color(n, v)
}
func (headerTextTheme) Font(s fyne.TextStyle) fyne.Resource {
	return fyne.CurrentApp().Settings().Theme().Font(s)
}
func (headerTextTheme) Icon(n fyne.ThemeIconName) fyne.Resource {
	return fyne.CurrentApp().Settings().Theme().Icon(n)
}
func (h headerTextTheme) Size(n fyne.ThemeSizeName) float32 {
	switch n {
	case theme.SizeNameInnerPadding:
		return 0
	case theme.SizeNameText:
		return h.size
	default:
		return fyne.CurrentApp().Settings().Theme().Size(n)
	}
}
