// Package refmanifest defines the optional ref manifest envelope (metadata + controls).
package refmanifest

import (
	"encoding/json"

	"github.com/display-protocol/dp1-go/extension/identity"
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
	Medium      *Medium              `json:"medium,omitempty"`
	Tags        []string             `json:"tags,omitempty"`
	Thumbnails  map[string]Thumbnail `json:"thumbnails,omitempty"`
}

// Medium says what the work is made of, twice over (spec §4.1, refVersion 1.2.0): Type is
// the filter a catalog groups by, Details the lines a reader sees under the title.
type Medium struct {
	// Type is one of image, video, audio, software, 3d, animated-gif, pdf, text, other.
	// Empty when the producer does not know it.
	Type string `json:"type,omitempty"`
	// Details are short labels in display order, one fact each (format with color and
	// sound, dimensions, duration, technique, toolchain); at most eight, each 1–200
	// characters. Consumers render them in order and must not reorder or parse them.
	Details []string `json:"details,omitempty"`
}

// Artist, Biography, Link and Thumbnail are the single Entity model of the DP-1 spec
// (ref-manifest $defs/Entity and friends), shared with the channels and playlists
// extensions, which name curators, publishers and note authors with the same shape.
//
// They are aliases of the extension/identity types rather than copies: two field-for-field
// mirrors had to be kept in step by hand, and one that lagged would validate a document and
// then silently drop what the schema accepted. An alias makes that drift impossible, and
// keeps existing refmanifest.Artist{...} code compiling. Add new Entity fields in package
// identity only.
//
// Identity: only Addresses may be used to recognize the same artist across producers. ID is
// producer-scoped and Name is display text; Addresses is a signed claim of the producer, so
// consumers must not merge two records merely because their address lists intersect.
type (
	// Artist is identity.Entity: a creator credited in metadata.artists[], and from
	// refVersion 1.1.0 a profile snapshot taken when the manifest was authored (spec §4.1).
	Artist = identity.Entity
	// Biography is one biographical text with optional attribution.
	Biography = identity.Biography
	// Link is an external link (profile or editorial); see identity.Link.
	Link = identity.Link
	// Thumbnail references a preview image; only URI is required (core changelog 2026-08-12).
	// The schema accepts integer spellings Go does not ("w": 1e2); that gap is tracked in #7.
	Thumbnail = identity.Thumbnail
)

// Link types accepted by the schema's links[].type enumeration; see identity.LinkType*.
const (
	LinkTypeWebsite     = identity.LinkTypeWebsite
	LinkTypeTwitter     = identity.LinkTypeTwitter
	LinkTypeInstagram   = identity.LinkTypeInstagram
	LinkTypeProfile     = identity.LinkTypeProfile
	LinkTypePublication = identity.LinkTypePublication
	LinkTypeArticle     = identity.LinkTypeArticle
	LinkTypeEvent       = identity.LinkTypeEvent
	LinkTypeNews        = identity.LinkTypeNews
	LinkTypeOther       = identity.LinkTypeOther
)

// Controls groups display and safety preferences and device permissions from the manifest.
type Controls struct {
	Display *DisplayControls `json:"display,omitempty"`
	Safety  *SafetyControls  `json:"safety,omitempty"`
	// Permissions lists the device capabilities the work needs to run as intended: "camera",
	// "microphone" (refVersion 1.2.0). A player that cannot grant one should not start the
	// work silently; it should show why the work cannot play.
	Permissions []string `json:"permissions,omitempty"`
}

// DisplayControls mirrors playlist display prefs at manifest level.
type DisplayControls struct {
	Scaling    string          `json:"scaling,omitempty"`
	Margin     json.RawMessage `json:"margin,omitempty"`
	Background string          `json:"background,omitempty"`
	Autoplay   *bool           `json:"autoplay,omitempty"`
	Loop       *bool           `json:"loop,omitempty"`
	// ImageRendering is "auto" or "pixelated", as playlist.DisplayPrefs.ImageRendering
	// (refVersion 1.2.0); merge layers it in the same resolution order as the other fields.
	ImageRendering string          `json:"imageRendering,omitempty"`
	Interaction    json.RawMessage `json:"interaction,omitempty"`
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
