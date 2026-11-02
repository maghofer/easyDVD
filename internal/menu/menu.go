// Package menu shares one renderer between fast previews and authored menus.
package menu

import (
	"encoding/xml"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/jpeg"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"

	"easydvd/internal/model"
	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

type Button struct {
	Name                string
	Rect                image.Rectangle
	Action              Action
	Selected, Activated string
	// Command is retained for callers supplying legacy DVD navigation fixtures.
	Command string
}
type Action struct {
	Kind   string
	Target int
}

func (b Button) DVDCommand(index int) string {
	switch b.Action.Kind {
	case "title":
		return fmt.Sprintf("g0 = 0; g3 = %d; jump title %d;", (index+1)*1024, b.Action.Target)
	case "playall":
		return "g0 = 1; g3 = 1024; jump title 1;"
	case "page":
		return fmt.Sprintf("g3 = 1024; jump menu %d;", b.Action.Target)
	}
	return b.Command
}

type Page struct {
	Number                                                    int
	Background, Preview, Highlight, Select, XML, LetterboxXML string
	Buttons                                                   []Button
}

func Escape(s string) string {
	var b strings.Builder
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}
func save(path string, img image.Image) error {
	f, e := os.Create(path)
	if e != nil {
		return e
	}
	e = png.Encode(f, img)
	ce := f.Close()
	if e != nil {
		return e
	}
	return ce
}
func rectangle(img draw.Image, r image.Rectangle, c color.Color, width int) {
	u := image.NewUniform(c)
	for _, edge := range []image.Rectangle{image.Rect(r.Min.X, r.Min.Y, r.Max.X, r.Min.Y+width), image.Rect(r.Min.X, r.Max.Y-width, r.Max.X, r.Max.Y), image.Rect(r.Min.X, r.Min.Y, r.Min.X+width, r.Max.Y), image.Rect(r.Max.X-width, r.Min.Y, r.Max.X, r.Max.Y)} {
		draw.Draw(img, edge, u, image.Point{}, draw.Src)
	}
}

type painter struct {
	regular, bold *opentype.Font
	faces         map[string]font.Face
}

func (p *painter) close() {
	for _, f := range p.faces {
		f.Close()
	}
}
func (p *painter) text(img draw.Image, x, y int, s string, size int, c color.Color, bold bool, maxWidth int) {
	key := fmt.Sprintf("%d:%v", size, bold)
	f := p.faces[key]
	if f == nil {
		ft := p.regular
		if bold {
			ft = p.bold
		}
		f, _ = opentype.NewFace(ft, &opentype.FaceOptions{Size: float64(size), DPI: 72, Hinting: font.HintingFull})
		p.faces[key] = f
	}
	d := font.Drawer{Dst: img, Src: image.NewUniform(c), Face: f, Dot: fixed.P(x, y)}
	if maxWidth > 0 && d.MeasureString(s).Ceil() > maxWidth {
		r := []rune(s)
		lo, hi := 0, len(r)
		for lo < hi {
			mid := (lo + hi + 1) / 2
			if d.MeasureString(string(r[:mid])+"…").Ceil() <= maxWidth {
				lo = mid
			} else {
				hi = mid - 1
			}
		}
		s = string(r[:lo]) + "…"
	}
	d.DrawString(s)
}

// Preview draws only the requested page in square display pixels. No PNGs,
// external tools, or other pages are produced in this path.
func Preview(title string, disc, total int, videos []model.Video, s model.Settings, page int) (image.Image, error) {
	img, _, err := drawPage(title, disc, total, videos, s, page)
	return img, err
}

func drawPage(title string, disc, total int, videos []model.Video, s model.Settings, page int) (*image.RGBA, []Button, error) {
	if err := s.Validate(); err != nil {
		return nil, nil, err
	}
	count := (len(videos) + s.TitlesPerPage - 1) / s.TitlesPerPage
	if page < 1 || page > count {
		return nil, nil, fmt.Errorf("menu page is out of range")
	}
	style := s.Menu.Defaults()
	unit := func(n int) int { return n }
	if s.IsBluray() {
		unit = func(n int) int { return int(math.Round(float64(n) * 1080 / 576)) }
		style.FontSize, style.HeadingSize, style.LineSpacing = unit(style.FontSize), unit(style.HeadingSize), unit(style.LineSpacing)
	}
	w := 768
	if style.Aspect == "16:9" {
		w = 1024
	}
	w = unit(w)
	h := unit(576)
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	bg := color.RGBA{22, 27, 34, 255}
	draw.Draw(img, img.Bounds(), image.NewUniform(bg), image.Point{}, draw.Src)
	if style.Background != "" {
		pic, err := background(style.Background)
		if err != nil {
			return nil, nil, fmt.Errorf("menu background: %w", err)
		}
		bounds := pic.Bounds()
		ratio := math.Max(float64(w)/float64(bounds.Dx()), float64(h)/float64(bounds.Dy()))
		dw, dh := int(math.Ceil(float64(bounds.Dx())*ratio)), int(math.Ceil(float64(bounds.Dy())*ratio))
		xdraw.BiLinear.Scale(img, image.Rect((w-dw)/2, (h-dh)/2, (w-dw)/2+dw, (h-dh)/2+dh), pic, bounds, draw.Over, nil)
		draw.Draw(img, img.Bounds(), image.NewUniform(color.NRGBA{0, 0, 0, uint8(style.Dimming * 255 / 100)}), image.Point{}, draw.Over)
	}
	regular, bold, err := typefaces(style)
	if err != nil {
		return nil, nil, err
	}
	p := painter{regular, bold, map[string]font.Face{}}
	defer p.close()
	fg := Color(style.FontColor)
	margin := w / 16
	right := w - margin
	p.text(img, margin, unit(66), title, style.HeadingSize, fg, true, right-margin)
	discWidth := 200
	if s.IsBluray() {
		discWidth = 230
	}
	p.text(img, margin, unit(94), fmt.Sprintf("%s %d of %d", s.DiscLabel(), disc, total), unit(16), fg, false, unit(discWidth))
	p.text(img, right-unit(130), unit(94), fmt.Sprintf("Page %d / %d", page, count), unit(16), fg, false, unit(130))
	draw.Draw(img, image.Rect(margin, unit(109), right, unit(111)), image.NewUniform(color.RGBA{70, 80, 94, 255}), image.Point{}, draw.Src)
	var buttons []Button
	start := (page - 1) * s.TitlesPerPage
	end := min(start+s.TitlesPerPage, len(videos))
	pitch := min(max(style.FontSize+style.LineSpacing, style.FontSize+unit(6)), unit(350)/s.TitlesPerPage)
	textSize := min(style.FontSize, pitch-unit(6))
	for i := start; i < end; i++ {
		row := i - start
		var r image.Rectangle
		if s.Layout == "Thumbnails" {
			cellW := (right - margin - unit(32)) / 3
			cellH := unit(174)
			x, y := margin+(row%3)*(cellW+unit(16)), unit(126)+(row/3)*cellH
			r = image.Rect(x, y, x+cellW, y+cellH-unit(12))
			thumb := image.Rect(x+unit(2), y+unit(2), x+cellW-unit(2), r.Max.Y-style.FontSize-unit(14))
			if f, e := os.Open(videos[i].Thumbnail); e == nil {
				pic, _, e := image.Decode(f)
				f.Close()
				if e == nil {
					fitImage(img, thumb, pic)
				}
			}
			rectangle(img, thumb, color.RGBA{70, 80, 94, 255}, unit(1))
			p.text(img, x+unit(7), r.Max.Y-unit(9), videos[i].Label, style.FontSize, fg, false, cellW-unit(14))
		} else {
			y := unit(126) + row*pitch
			r = image.Rect(margin, y, right, y+pitch-unit(3))
			p.text(img, margin+unit(14), y+(pitch+textSize)/2-unit(3), videos[i].Label, textSize, fg, false, right-margin-unit(28))
		}
		buttons = append(buttons, Button{Name: fmt.Sprintf("title%03d", i+1), Rect: r, Action: Action{"title", i + 1}})
	}
	add := func(name, label string, x, width int, action Action) {
		r := image.Rect(x, unit(496), x+width, unit(532))
		p.text(img, x+unit(12), unit(520), label, unit(18), fg, false, width-unit(20))
		buttons = append(buttons, Button{Name: name, Rect: r, Action: action})
	}
	add("playall", "Play All", margin, unit(170), Action{"playall", 0})
	if page > 1 {
		add("previous", "Previous", right-unit(294), unit(140), Action{"page", page - 1})
	}
	if page < count {
		add("next", "Next", right-unit(132), unit(132), Action{"page", page + 1})
	}
	if s.IsBluray() && w != 1920 {
		wide := image.NewRGBA(image.Rect(0, 0, 1920, 1080))
		draw.Draw(wide, wide.Bounds(), image.NewUniform(color.Black), image.Point{}, draw.Src)
		offset := (1920 - w) / 2
		draw.Draw(wide, image.Rect(offset, 0, offset+w, h), img, image.Point{}, draw.Src)
		for i := range buttons {
			buttons[i].Rect = buttons[i].Rect.Add(image.Pt(offset, 0))
		}
		img = wide
	}
	return img, buttons, nil
}

func fitImage(dst draw.Image, r image.Rectangle, pic image.Image) {
	ratio := math.Min(float64(r.Dx())/float64(pic.Bounds().Dx()), float64(r.Dy())/float64(pic.Bounds().Dy()))
	w, h := int(float64(pic.Bounds().Dx())*ratio), int(float64(pic.Bounds().Dy())*ratio)
	x, y := r.Min.X+(r.Dx()-w)/2, r.Min.Y+(r.Dy()-h)/2
	xdraw.BiLinear.Scale(dst, image.Rect(x, y, x+w, y+h), pic, pic.Bounds(), draw.Over, nil)
}

func Render(dir, title string, disc, total int, videos []model.Video, s model.Settings) ([]Page, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	var pages []Page
	for n := 1; n <= (len(videos)+s.TitlesPerPage-1)/s.TitlesPerPage; n++ {
		img, buttons, err := drawPage(title, disc, total, videos, s, n)
		if err != nil {
			return nil, err
		}
		stem := filepath.Join(dir, fmt.Sprintf("page-%03d", n))
		page := Page{Number: n, Background: stem + ".png", Preview: stem + "-preview.png", Highlight: stem + "-highlight.png", Select: stem + "-select.png", XML: stem + ".xml"}
		if s.IsBluray() {
			page.Buttons = buttons
			if err = save(page.Background, img); err != nil {
				return nil, err
			}
			if err = save(page.Preview, img); err != nil {
				return nil, err
			}
			for i := range page.Buttons {
				bt := &page.Buttons[i]
				bounds := image.Rect(0, 0, bt.Rect.Dx(), bt.Rect.Dy())
				for state, c := range map[string]color.Color{"highlight": Color(s.Menu.Defaults().HighlightColor), "activate": color.RGBA{255, 211, 127, 255}} {
					mask := image.NewNRGBA(bounds)
					rectangle(mask, bounds, c, 4)
					path := stem + "-" + bt.Name + "-" + state + ".png"
					if err = save(path, mask); err != nil {
						return nil, err
					}
					if state == "highlight" {
						bt.Selected = path
					} else {
						bt.Activated = path
					}
				}
			}
			pages = append(pages, page)
			continue
		}
		raster := image.NewRGBA(image.Rect(0, 0, 720, s.Height()))
		xdraw.BiLinear.Scale(raster, raster.Bounds(), img, img.Bounds(), draw.Src, nil)
		for _, bt := range buttons {
			r := bt.Rect
			r.Min.X = r.Min.X * 720 / img.Bounds().Dx()
			r.Max.X = r.Max.X * 720 / img.Bounds().Dx()
			r.Min.Y = (r.Min.Y * s.Height() / 576) / 2 * 2
			r.Max.Y = (r.Max.Y * s.Height() / 576) / 2 * 2
			bt.Rect = r
			page.Buttons = append(page.Buttons, bt)
		}
		if err = save(page.Background, raster); err != nil {
			return nil, err
		}
		if err = save(page.Preview, img); err != nil {
			return nil, err
		}
		if err = writeMasks(page, s, false); err != nil {
			return nil, err
		}
		if s.Menu.Defaults().Aspect == "16:9" {
			page.LetterboxXML = stem + "-letterbox.xml"
			if err = writeMasks(page, s, true); err != nil {
				return nil, err
			}
		}
		pages = append(pages, page)
	}
	return pages, nil
}

func writeMasks(page Page, s model.Settings, letterbox bool) error {
	hiPath, selPath, xmlPath := page.Highlight, page.Select, page.XML
	if letterbox {
		hiPath = strings.TrimSuffix(page.Highlight, ".png") + "-letterbox.png"
		selPath = strings.TrimSuffix(page.Select, ".png") + "-letterbox.png"
		xmlPath = page.LetterboxXML
	}
	bounds := image.Rect(0, 0, 720, s.Height())
	style := s.Menu.Defaults()
	hi := image.NewPaletted(bounds, color.Palette{color.RGBA{}, Color(style.HighlightColor)})
	sel := image.NewPaletted(bounds, color.Palette{color.RGBA{}, color.RGBA{255, 211, 127, 255}})
	var b strings.Builder
	fmt.Fprintf(&b, "<subpictures><stream><spu start=\"00:00:00.00\" highlight=\"%s\" select=\"%s\" force=\"yes\">\n", Escape(filepath.Base(hiPath)), Escape(filepath.Base(selPath)))
	for k, bt := range page.Buttons {
		r := bt.Rect
		if letterbox {
			r.Min.Y = (r.Min.Y*3/4 + s.Height()/8) / 2 * 2
			r.Max.Y = (r.Max.Y*3/4 + s.Height()/8) / 2 * 2
		}
		rectangle(hi, r, hi.Palette[1], 2)
		rectangle(sel, r, sel.Palette[1], 2)
		prev, next := (k+len(page.Buttons)-1)%len(page.Buttons), (k+1)%len(page.Buttons)
		fmt.Fprintf(&b, "<button name=\"%s\" x0=\"%d\" y0=\"%d\" x1=\"%d\" y1=\"%d\" up=\"%s\" down=\"%s\" left=\"%s\" right=\"%s\"/>\n", bt.Name, r.Min.X, r.Min.Y, r.Max.X-1, r.Max.Y-1, page.Buttons[prev].Name, page.Buttons[next].Name, page.Buttons[prev].Name, page.Buttons[next].Name)
	}
	b.WriteString("</spu></stream></subpictures>\n")
	if err := save(hiPath, hi); err != nil {
		return err
	}
	if err := save(selPath, sel); err != nil {
		return err
	}
	return os.WriteFile(xmlPath, []byte(b.String()), 0644)
}
