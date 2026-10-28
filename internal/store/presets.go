package store

import (
	"easydvd/internal/model"
	"encoding/json"
	"fmt"
	"strings"
)

func (s *Store) MenuPresets() ([]model.MenuPreset, error) {
	rows, err := s.db.Query(`SELECT payload FROM menu_presets ORDER BY name COLLATE NOCASE`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var presets []model.MenuPreset
	for rows.Next() {
		var b []byte
		var p model.MenuPreset
		if err = rows.Scan(&b); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(b, &p); err != nil {
			return nil, err
		}
		presets = append(presets, p)
	}
	return presets, rows.Err()
}
func (s *Store) SaveMenuPreset(p model.MenuPreset) error {
	p.Name = strings.TrimSpace(p.Name)
	if p.Name == "" || len([]rune(p.Name)) > 80 {
		return fmt.Errorf("preset name must contain 1–80 characters")
	}
	if err := p.Style.Validate(); err != nil {
		return err
	}
	if (p.Layout != "Text" && p.Layout != "Thumbnails") || p.TitlesPerPage < 1 || p.TitlesPerPage > 12 || (p.Layout == "Thumbnails" && p.TitlesPerPage > 6) {
		return fmt.Errorf("invalid preset layout or titles per page")
	}
	b, err := json.Marshal(p)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`INSERT INTO menu_presets(name,payload) VALUES(?,?) ON CONFLICT(name) DO UPDATE SET payload=excluded.payload`, p.Name, b)
	return err
}
