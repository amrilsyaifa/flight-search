package mockdata

import (
	"encoding/json"
	"testing"
)

// Catches empty or broken mock files at test time instead of at runtime.
func TestEmbeddedFilesAreValidJSON(t *testing.T) {
	files := map[string][]byte{
		"GarudaIndonesia": GarudaIndonesia,
		"LionAir":         LionAir,
		"BatikAir":        BatikAir,
		"AirAsia":         AirAsia,
	}
	for name, data := range files {
		if len(data) == 0 {
			t.Errorf("%s: embedded file is empty", name)
			continue
		}
		if !json.Valid(data) {
			t.Errorf("%s: embedded file is not valid JSON", name)
		}
	}
}
