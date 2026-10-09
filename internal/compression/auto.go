package compression

import "fmt"

// AutoSettings controls the optional lossy auto profile only.
type AutoSettings struct {
	JPEGQuality     int `json:"jpeg_quality"`
	MinSavedPercent int `json:"min_saved_percent"`
	DPI             int `json:"dpi"`
}

func DefaultAutoSettings() AutoSettings {
	return AutoSettings{JPEGQuality: 70, MinSavedPercent: 20, DPI: 200}
}
func (s AutoSettings) Validate() error {
	if s.JPEGQuality < 0 || s.JPEGQuality > 100 || s.MinSavedPercent < 0 || s.MinSavedPercent > 100 || (s.DPI != 150 && s.DPI != 200) {
		return fmt.Errorf("invalid auto settings")
	}
	return nil
}
