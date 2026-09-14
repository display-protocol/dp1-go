package refmanifest

import (
	"encoding/json"
	"testing"
)

func TestManifest_JSONRoundTrip(t *testing.T) {
	t.Parallel()
	m := Manifest{
		RefVersion: "0.1.0",
		ID:         "r",
		Created:    "2025-01-01T00:00:00Z",
		Locale:     "en",
		Metadata: &Metadata{
			Title: "X",
			Artists: []Artist{{
				Name:        "N",
				Addresses:   []string{"0xabc"},
				Avatar:      &Thumbnail{URI: "https://a.example/n.jpg"},
				Biographies: []Biography{{Text: "Bio", Source: "S", SourceURL: "https://s.example"}},
				Links:       []Link{{Type: LinkTypeWebsite, URL: "https://n.example"}},
			}},
		},
		Controls: &Controls{
			Safety: &SafetyControls{
				Orientation: []string{"any"},
				MaxCPUPct:   intPtr(50),
			},
		},
		I18n: map[string]LocalizedMetadata{
			"ja": {Title: "あ"},
		},
	}
	b, err := json.Marshal(&m)
	if err != nil {
		t.Fatal(err)
	}
	var out Manifest
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	if out.Metadata.Title != "X" {
		t.Fatal(out.Metadata.Title)
	}
	a := out.Metadata.Artists[0]
	if a.Addresses[0] != "0xabc" || a.Avatar.URI != "https://a.example/n.jpg" ||
		a.Biographies[0].SourceURL != "https://s.example" || a.Links[0].Type != LinkTypeWebsite {
		t.Fatalf("artist profile did not round-trip: %+v", a)
	}
}

func intPtr(i int) *int { return &i }
