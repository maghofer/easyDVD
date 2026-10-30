package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// Diagnostic text is read-only, but it should have the contrast of normal
// text rather than the dim color used for unavailable controls.
func newReadOnlyText(text string) (*widget.Entry, *container.ThemeOverride) {
	entry := widget.NewMultiLineEntry()
	entry.SetText(text)
	entry.Disable()
	return entry, container.NewThemeOverride(entry, readableTextTheme{fyne.CurrentApp().Settings().Theme()})
}

type readableTextTheme struct{ fyne.Theme }

func (t readableTextTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	if name == theme.ColorNameDisabled {
		name = theme.ColorNameForeground
	}
	return t.Theme.Color(name, variant)
}
