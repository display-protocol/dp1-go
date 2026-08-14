// Package refmanifest defines the optional ref manifest envelope (metadata + controls).
package refmanifest

import "encoding/json"

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
//
// Note the schema accepts integer spellings Go does not: "w": 1e2 and 100.0 validate but fail
// to decode into int. That gap predates the relaxation and is tracked in #7.
type Thumbnail struct {
	URI    string `json:"uri"`
	W      *int   `json:"w,omitempty"`
	H      *int   `json:"h,omitempty"`
	SHA256 string `json:"sha256,omitempty"`
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
