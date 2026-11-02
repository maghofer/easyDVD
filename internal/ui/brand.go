package ui

import (
	_ "embed"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/theme"
)

//go:embed assets/logo-bluray-correction.png
var headerLogo []byte

//go:embed assets/dvd-logo.svg
var dvdLogo []byte

//go:embed assets/bluray-logo.svg
var blurayLogo []byte

//go:embed assets/app.png
var appIcon []byte

//go:embed assets/media-tools.svg
var mediaToolsSVG []byte

var mediaToolsIcon = theme.NewThemedResource(fyne.NewStaticResource("media-tools.svg", mediaToolsSVG))

func applicationIcon() fyne.Resource { return fyne.NewStaticResource("app.png", appIcon) }

func newBrandLogo() *canvas.Image {
	logo := canvas.NewImageFromResource(fyne.NewStaticResource("logo-bluray-correction.png", headerLogo))
	logo.FillMode = canvas.ImageFillContain
	logo.SetMinSize(fyne.NewSize(192, 64))
	return logo
}
