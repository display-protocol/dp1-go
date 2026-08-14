// Package refmanifest defines the optional ref manifest envelope (metadata + controls).
package refmanifest

import (
	"encoding/json"
	"fmt"
	"math"
)

// Manifest is the ref manifest root (DP-1 ref-manifest schema).
type Manifest struct {
	RefVersion string                       `json:"refVersion"`
	ID         string                       `json:"id"`
	Created    string                       `json:"created"`
	Locale     string                       `json:"locale"`
	Metadata   *Metadata                    `json:"metadata,omitempty"`
	Controls   *Controls                    `json:"controls,omitempty"`
	I18n       map[string]LocalizedMetadata `json:"i18n,omitempty"`
}

// Metadata carries human-facing labels and media.
type Metadata struct {
	Title       string               `json:"title,omitempty"`
	Artists     []Artist             `json:"artists,omitempty"`
	CreditLine  string               `json:"creditLine,omitempty"`
	Description string               `json:"description,omitempty"`
	Tags        []string             `json:"tags,omitempty"`
	Thumbnails  map[string]Thumbnail `json:"thumbnails,omitempty"`
}

// Artist identifies a creator.
type Artist struct {
	Name string `json:"name"`
	ID   string `json:"id,omitempty"`
	URL  string `json:"url,omitempty"`
}

// Thumbnail references a preview image.
//
// Only URI is required. W and H are pointers because the ref-manifest schema relaxed
// `required` from ["uri","w","h"] to ["uri"] (core changelog 2026-08-12): producers that
// hold only a bare thumbnail URL omit the dimensions rather than guess, and consumers MUST
// treat them as possibly absent. A nil pointer means "unknown", which a plain int could not
// distinguish from a value the emitter actually wrote.
type Thumbnail struct {
	URI    string `json:"uri"`
	W      *int   `json:"w,omitempty"`
	H      *int   `json:"h,omitempty"`
	SHA256 string `json:"sha256,omitempty"`
}

// UnmarshalJSON decodes a thumbnail, accepting every JSON spelling of an integer that the
// schema does.
//
// JSON Schema "integer" means a mathematical integer, not a syntax: 100, 1e2 and 100.0 are all
// valid widths, and a producer whose numbers are floats emits the latter forms routinely.
// encoding/json refuses to put any of them but the first into an int, which would leave a
// schema-valid manifest — and, since these can arrive inline, a whole conforming playlist —
// undecodable. Validation and decoding have to accept the same numeric language.
//
// The equivalent gap remains on the SDK's other numeric fields; only the thumbnail dimensions
// are in scope here, and they are the fields the relaxation of `required` newly exposes.
func (t *Thumbnail) UnmarshalJSON(data []byte) error {
	// The alias sheds this method, so the embedded value decodes with the default rules while
	// the shadowing w/h fields at depth 0 capture the raw numbers for the tolerant path.
	type alias Thumbnail
	var raw struct {
		alias
		W json.RawMessage `json:"w"`
		H json.RawMessage `json:"h"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	w, err := parseDimension(raw.W)
	if err != nil {
		return fmt.Errorf("thumbnail w: %w", err)
	}
	h, err := parseDimension(raw.H)
	if err != nil {
		return fmt.Errorf("thumbnail h: %w", err)
	}
	*t = Thumbnail(raw.alias)
	t.W, t.H = w, h
	return nil
}

// parseDimension reads one optional pixel dimension. Absent and null both mean unknown.
// A number that is not a mathematical integer is rejected, as is one outside int — the schema
// bounds dimensions only from below (minimum 1), so an oversized value is representable in
// JSON but not in Go, and silently truncating it would invent a size.
func parseDimension(raw json.RawMessage) (*int, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	// json.Number accepts a quoted string too; the schema does not, and being laxer than the
	// document it validates against is how a producer's bug reaches a player unnoticed.
	if raw[0] == '"' {
		return nil, fmt.Errorf("%s is a string, want a number", raw)
	}
	var n json.Number
	if err := json.Unmarshal(raw, &n); err != nil {
		return nil, err
	}
	if i, err := n.Int64(); err == nil {
		if i < math.MinInt || i > math.MaxInt {
			return nil, fmt.Errorf("%s out of range for int", n)
		}
		v := int(i)
		return &v, nil
	}
	f, err := n.Float64()
	if err != nil {
		return nil, err
	}
	if math.IsInf(f, 0) || math.IsNaN(f) || f != math.Trunc(f) {
		return nil, fmt.Errorf("%s is not an integer", n)
	}
	if f < math.MinInt || f > math.MaxInt {
		return nil, fmt.Errorf("%s out of range for int", n)
	}
	v := int(f)
	return &v, nil
}

// Controls groups display and safety preferences from the manifest.
type Controls struct {
	Display *DisplayControls `json:"display,omitempty"`
	Safety  *SafetyControls  `json:"safety,omitempty"`
}

// DisplayControls mirrors playlist display prefs at manifest level.
type DisplayControls struct {
	Scaling     string          `json:"scaling,omitempty"`
	Margin      json.RawMessage `json:"margin,omitempty"`
	Background  string          `json:"background,omitempty"`
	Autoplay    *bool           `json:"autoplay,omitempty"`
	Loop        *bool           `json:"loop,omitempty"`
	Interaction json.RawMessage `json:"interaction,omitempty"`
}

// SafetyConstraints limits runtime resources.
type SafetyControls struct {
	Orientation []string `json:"orientation,omitempty"`
	MaxCPUPct   *int     `json:"maxCpuPct,omitempty"`
	MaxMemMB    *int     `json:"maxMemMB,omitempty"`
}

// LocalizedMetadata holds translated strings for one locale key.
type LocalizedMetadata struct {
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	CreditLine  string `json:"creditLine,omitempty"`
}
