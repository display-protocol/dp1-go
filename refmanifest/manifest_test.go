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
			Title:   "X",
			Artists: []Artist{{Name: "N"}},
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
}

func intPtr(i int) *int { return &i }

// Thumbnail decodes every JSON spelling of an integer the schema calls valid, and nothing more.
func TestThumbnail_UnmarshalJSON_dimensions(t *testing.T) {
	t.Parallel()

	accepted := map[string]*int{
		`{"uri":"u"}`:                nil,
		`{"uri":"u","w":null}`:       nil,
		`{"uri":"u","w":100}`:        intPtr(100),
		`{"uri":"u","w":1e2}`:        intPtr(100),
		`{"uri":"u","w":100.0}`:      intPtr(100),
		`{"uri":"u","w":1.2e3}`:      intPtr(1200),
		`{"uri":"u","w":0}`:          intPtr(0), // schema's minimum rejects it; decoding is not the gate
		`{"uri":"u","w":-100}`:       intPtr(-100),
		`{"uri":"u","w":1e2,"h":90}`: intPtr(100),
	}
	for doc, want := range accepted {
		t.Run("ok:"+doc, func(t *testing.T) {
			t.Parallel()
			var th Thumbnail
			if err := json.Unmarshal([]byte(doc), &th); err != nil {
				t.Fatalf("%s: %v", doc, err)
			}
			if th.URI != "u" {
				t.Fatalf("uri lost: %+v", th)
			}
			switch {
			case want == nil && th.W != nil:
				t.Fatalf("w = %d, want absent", *th.W)
			case want != nil && (th.W == nil || *th.W != *want):
				t.Fatalf("w = %v, want %d", th.W, *want)
			}
		})
	}

	rejected := []string{
		`{"uri":"u","w":"1200"}`, // string: json.Number would take it, the schema would not
		`{"uri":"u","w":100.5}`,  // not a mathematical integer
		`{"uri":"u","w":true}`,
		`{"uri":"u","w":{}}`,
		`{"uri":"u","w":[]}`,
		`{"uri":"u","w":1e30}`,  // integral, but outside int
		`{"uri":"u","w":1e400}`, // outside float64 entirely
		`{"uri":"u","h":"90"}`,  // the h path is checked independently of w
		`{"uri":"u"`,            // malformed object
	}
	for _, doc := range rejected {
		t.Run("reject:"+doc, func(t *testing.T) {
			t.Parallel()
			var th Thumbnail
			if err := json.Unmarshal([]byte(doc), &th); err == nil {
				t.Fatalf("%s must be rejected, got %+v", doc, th)
			}
		})
	}
}

// The custom decoder must not disturb the other fields, and must still round-trip.
func TestThumbnail_UnmarshalJSON_keepsSiblingFieldsAndRoundTrips(t *testing.T) {
	t.Parallel()
	const sha = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	var th Thumbnail
	if err := json.Unmarshal([]byte(`{"uri":"ipfs://x","w":1e2,"h":90,"sha256":"`+sha+`"}`), &th); err != nil {
		t.Fatal(err)
	}
	if th.URI != "ipfs://x" || th.SHA256 != sha {
		t.Fatalf("sibling fields lost: %+v", th)
	}
	b, err := json.Marshal(&th)
	if err != nil {
		t.Fatal(err)
	}
	// Re-encoding normalizes 1e2 to 100 — the value is preserved, the spelling is not.
	if string(b) != `{"uri":"ipfs://x","w":100,"h":90,"sha256":"`+sha+`"}` {
		t.Fatalf("round trip: %s", b)
	}
}

// Thumbnails arrive as map values, where the decoder is reached through a different path.
func TestThumbnail_UnmarshalJSON_insideMetadataMap(t *testing.T) {
	t.Parallel()
	var m Manifest
	doc := `{"refVersion":"0.1.0","id":"r","created":"2026-07-28T00:00:00Z","locale":"en",
		"metadata":{"thumbnails":{"default":{"uri":"u","w":1e2},"small":{"uri":"s"}}}}`
	if err := json.Unmarshal([]byte(doc), &m); err != nil {
		t.Fatal(err)
	}
	if w := m.Metadata.Thumbnails["default"].W; w == nil || *w != 100 {
		t.Fatalf("map value decoder not applied: %v", w)
	}
	if th := m.Metadata.Thumbnails["small"]; th.W != nil || th.H != nil {
		t.Fatalf("absent dimensions became %v/%v", th.W, th.H)
	}
}
