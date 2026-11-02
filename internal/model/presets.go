package model

import (
	"fmt"
	"strings"
)

// Encoding presets deliberately exclude application workers and menu artwork.
type EncodingPreset struct {
	Name            string `json:"name"`
	DiscFormat      string `json:"disc_format"`
	Standard        string `json:"standard"`
	Media           string `json:"media"`
	EncodingProfile string `json:"encoding_profile"`
}

func (p EncodingPreset) Apply(s *Settings) {
	s.DiscFormat, s.Standard, s.Media, s.EncodingProfile = p.DiscFormat, p.Standard, p.Media, p.EncodingProfile
}
func (p EncodingPreset) Validate() error {
	if strings.TrimSpace(p.Name) == "" || len([]rune(p.Name)) > 80 {
		return fmt.Errorf("preset name must contain 1–80 characters")
	}
	s := DefaultSettings("/")
	p.Apply(&s)
	if s.Format() != "DVD" && !s.IsBluray() {
		return fmt.Errorf("choose DVD or Blu-ray")
	}
	if s.Standard != "PAL" && s.Standard != "NTSC" {
		return fmt.Errorf("choose PAL or NTSC")
	}
	if s.IsBluray() {
		if s.Media != "BD-25" && s.Media != "BD-50" {
			return fmt.Errorf("choose BD-25 or BD-50")
		}
	} else if s.Media != "DVD-5" && s.Media != "DVD-9" {
		return fmt.Errorf("choose DVD-5 or DVD-9")
	}
	if s.Profile() != ProfileBalanced && s.Profile() != ProfileQuality {
		return fmt.Errorf("choose Balanced or Quality")
	}
	return nil
}
