package model

import (
	"fmt"
	"regexp"
)

type MenuStyle struct {
	Aspect         string `json:"aspect"`
	Background     string `json:"background,omitempty"`
	BackgroundName string `json:"background_name,omitempty"`
	Font           string `json:"font"`
	FontFile       string `json:"font_file,omitempty"`
	FontName       string `json:"font_name,omitempty"`
	FontColor      string `json:"font_color"`
	FontSize       int    `json:"font_size"`
	HeadingSize    int    `json:"heading_size"`
	LineSpacing    int    `json:"line_spacing"`
	Dimming        int    `json:"dimming"`
	HighlightColor string `json:"highlight_color"`
}

func DefaultMenuStyle() MenuStyle {
	return MenuStyle{Aspect: "4:3", Font: "Sans", FontColor: "#ECF0F6", FontSize: 20, HeadingSize: 28, LineSpacing: 12, Dimming: 55, HighlightColor: "#7DB5F6"}
}

// Old saved queues have no style. Preserve their working defaults.
func (m MenuStyle) Defaults() MenuStyle {
	if m == (MenuStyle{}) {
		return DefaultMenuStyle()
	}
	if m.Aspect == "" {
		m.Aspect = "4:3"
	}
	if m.Font == "" {
		m.Font = "Sans"
	}
	if m.FontColor == "" {
		m.FontColor = "#ECF0F6"
	}
	if m.HighlightColor == "" {
		m.HighlightColor = "#7DB5F6"
	}
	if m.FontSize == 0 {
		m.FontSize = 20
	}
	if m.HeadingSize == 0 {
		m.HeadingSize = 28
	}
	return m
}

var menuColor = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

func (m MenuStyle) Validate() error {
	m = m.Defaults()
	if m.Aspect != "4:3" && m.Aspect != "16:9" {
		return fmt.Errorf("choose 4:3 or 16:9 menus")
	}
	if m.Font != "Sans" && m.Font != "Monospace" && m.Font != "Custom" {
		return fmt.Errorf("choose a menu font")
	}
	if m.Font == "Custom" && m.FontFile == "" {
		return fmt.Errorf("choose a TTF or OTF font file")
	}
	if !menuColor.MatchString(m.FontColor) || !menuColor.MatchString(m.HighlightColor) {
		return fmt.Errorf("colors must use #RRGGBB notation")
	}
	if m.FontSize < 12 || m.FontSize > 36 || m.HeadingSize < 18 || m.HeadingSize > 44 {
		return fmt.Errorf("title font size must be 12–36; heading size 18–44")
	}
	if m.LineSpacing < 0 || m.LineSpacing > 30 {
		return fmt.Errorf("line spacing must be 0–30 pixels")
	}
	if m.Dimming < 0 || m.Dimming > 90 {
		return fmt.Errorf("background dimming must be 0–90 percent")
	}
	return nil
}

type MenuPreset struct {
	Name          string    `json:"name"`
	Style         MenuStyle `json:"style"`
	Layout        string    `json:"layout"`
	TitlesPerPage int       `json:"titles_per_page"`
}
