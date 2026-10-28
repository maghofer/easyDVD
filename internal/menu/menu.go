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
	Name    string
	Rect    image.Rectangle
	Command string
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
	w := 768
	if style.Aspect == "16:9" {
		w = 1024
	}
	img := image.NewRGBA(image.Rect(0, 0, w, 576))
	bg := color.RGBA{22, 27, 34, 255}
	draw.Draw(img, img.Bounds(), image.NewUniform(bg), image.Point{}, draw.Src)
	if style.Background != "" {
		pic, err := background(style.Background)
		if err != nil {
			return nil, nil, fmt.Errorf("menu background: %w", err)
		}
		bounds := pic.Bounds()
		ratio := math.Max(float64(w)/float64(bounds.Dx()), 576.0/float64(bounds.Dy()))
		dw, dh := int(math.Ceil(float64(bounds.Dx())*ratio)), int(math.Ceil(float64(bounds.Dy())*ratio))
		xdraw.BiLinear.Scale(img, image.Rect((w-dw)/2, (576-dh)/2, (w-dw)/2+dw, (576-dh)/2+dh), pic, bounds, draw.Over, nil)
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
	p.text(img, margin, 66, title, style.HeadingSize, fg, true, right-margin)
	p.text(img, margin, 94, fmt.Sprintf("DVD %d of %d", disc, total), 16, fg, false, 200)
	p.text(img, right-130, 94, fmt.Sprintf("Page %d / %d", page, count), 16, fg, false, 130)
	draw.Draw(img, image.Rect(margin, 109, right, 111), image.NewUniform(color.RGBA{70, 80, 94, 255}), image.Point{}, draw.Src)
	var buttons []Button
	start := (page - 1) * s.TitlesPerPage
	end := min(start+s.TitlesPerPage, len(videos))
	pitch := min(max(style.FontSize+style.LineSpacing, style.FontSize+6), 350/s.TitlesPerPage)
	textSize := min(style.FontSize, pitch-6)
	for i := start; i < end; i++ {
		row := i - start
		var r image.Rectangle
		if s.Layout == "Thumbnails" {
			cellW := (right - margin - 32) / 3
			cellH := 174
			x, y := margin+(row%3)*(cellW+16), 126+(row/3)*cellH
			r = image.Rect(x, y, x+cellW, y+cellH-12)
			thumb := image.Rect(x+2, y+2, x+cellW-2, r.Max.Y-style.FontSize-14)
			if f, e := os.Open(videos[i].Thumbnail); e == nil {
				pic, _, e := image.Decode(f)
				f.Close()
				if e == nil {
					fitImage(img, thumb, pic)
				}
			}
			rectangle(img, thumb, color.RGBA{70, 80, 94, 255}, 1)
			p.text(img, x+7, r.Max.Y-9, videos[i].Label, style.FontSize, fg, false, cellW-14)
		} else {
			y := 126 + row*pitch
			r = image.Rect(margin, y, right, y+pitch-3)
			p.text(img, margin+14, y+(pitch+textSize)/2-3, videos[i].Label, textSize, fg, false, right-margin-28)
		}
		buttons = append(buttons, Button{fmt.Sprintf("title%03d", i+1), r, fmt.Sprintf("g0 = 0; g3 = %d; jump title %d;", (len(buttons)+1)*1024, i+1)})
	}
	add := func(name, label string, x, width int, command string) {
		r := image.Rect(x, 496, x+width, 532)
		p.text(img, x+12, 520, label, 18, fg, false, width-20)
		buttons = append(buttons, Button{name, r, command})
	}
	add("playall", "Play All", margin, 170, "g0 = 1; g3 = 1024; jump title 1;")
	if page > 1 {
		add("previous", "Previous", right-294, 140, fmt.Sprintf("g3 = 1024; jump menu %d;", page-1))
	}
	if page < count {
		add("next", "Next", right-132, 132, fmt.Sprintf("g3 = 1024; jump menu %d;", page+1))
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
