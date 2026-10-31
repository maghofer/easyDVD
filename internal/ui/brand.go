package ui

import (
	_ "embed"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/theme"
)

//go:embed assets/logo-retro-gold.png
var retroGoldLogo []byte

//go:embed assets/app.png
var appIcon []byte

//go:embed assets/media-tools.svg
var mediaToolsSVG []byte

var mediaToolsIcon = theme.NewThemedResource(fyne.NewStaticResource("media-tools.svg", mediaToolsSVG))

func applicationIcon() fyne.Resource { return fyne.NewStaticResource("app.png", appIcon) }

func newBrandLogo() *canvas.Image {
	logo := canvas.NewImageFromResource(fyne.NewStaticResource("logo-retro-gold.png", retroGoldLogo))
	logo.FillMode = canvas.ImageFillContain
	logo.SetMinSize(fyne.NewSize(192, 64))
	return logo
}
