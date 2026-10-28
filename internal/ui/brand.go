package ui

import (
	_ "embed"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
)

//go:embed assets/logo-retro-gold.png
var retroGoldLogo []byte

//go:embed assets/app.png
var appIcon []byte

func applicationIcon() fyne.Resource { return fyne.NewStaticResource("app.png", appIcon) }

func newBrandLogo() *canvas.Image {
	logo := canvas.NewImageFromResource(fyne.NewStaticResource("logo-retro-gold.png", retroGoldLogo))
	logo.FillMode = canvas.ImageFillContain
	logo.SetMinSize(fyne.NewSize(192, 64))
	return logo
}
