// Package refmanifest defines the optional ref manifest envelope (metadata + controls).
package refmanifest

import (
	"encoding/json"

	"github.com/display-protocol/dp1-go/extension/identity"
	"github.com/display-protocol/dp1-go/internal/jsonint"
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

// UnmarshalJSON matches the "controls" member by exact name. encoding/json would read a
// schema-valid unknown "Controls" member as the recognized one, and merge would then apply its
// empty keyboard list as a revocation of inherited controls. Receiver values are kept for an
// absent member and null clears it, as the default decoder does.
func (m *Manifest) UnmarshalJSON(data []byte) error {
	type plain Manifest // strips this method to avoid recursion
	shadow := struct {
		plain
		Controls json.RawMessage `json:"controls"` // sink; exact lookup follows
	}{plain: plain(*m)}
	if err := json.Unmarshal(data, &shadow); err != nil {
		return err
	}
	out := Manifest(shadow.plain)
	var members map[string]json.RawMessage
	if err := json.Unmarshal(data, &members); err != nil {
		return nil //nolint:nilerr // shadow decode is authoritative for document-shape errors
	}
	if raw, ok := members["controls"]; ok {
		if err := json.Unmarshal(raw, &out.Controls); err != nil {
			return err
		}
	}
	*m = out
	return nil
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

// UnmarshalJSON matches the "display" member by exact name; see [Manifest.UnmarshalJSON].
func (c *Controls) UnmarshalJSON(data []byte) error {
	type plain Controls
	shadow := struct {
		plain
		Display json.RawMessage `json:"display"` // sink; exact lookup follows
	}{plain: plain(*c)}
	if err := json.Unmarshal(data, &shadow); err != nil {
		return err
	}
	out := Controls(shadow.plain)
	var members map[string]json.RawMessage
	if err := json.Unmarshal(data, &members); err != nil {
		return nil //nolint:nilerr // shadow decode is authoritative for document-shape errors
	}
	if raw, ok := members["display"]; ok {
		if err := json.Unmarshal(raw, &out.Display); err != nil {
			return err
		}
	}
	*c = out
	return nil
}

// UnmarshalJSON matches the "interaction" member by exact name; see [Manifest.UnmarshalJSON].
// The block stays raw (merge decodes it), so a present member replaces the seeded bytes.
func (d *DisplayControls) UnmarshalJSON(data []byte) error {
	type plain DisplayControls
	shadow := struct {
		plain
		Interaction json.RawMessage `json:"interaction"` // sink; exact lookup follows
	}{plain: plain(*d)}
	if err := json.Unmarshal(data, &shadow); err != nil {
		return err
	}
	out := DisplayControls(shadow.plain)
	var members map[string]json.RawMessage
	if err := json.Unmarshal(data, &members); err != nil {
		return nil //nolint:nilerr // shadow decode is authoritative for document-shape errors
	}
	if raw, ok := members["interaction"]; ok {
		out.Interaction = append(json.RawMessage(nil), raw...)
	}
	*d = out
	return nil
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

// UnmarshalJSON accepts every spelling of maxCpuPct and maxMemMB the schema's integer
// accepts; see [identity.Thumbnail.UnmarshalJSON], including the note on embedding.
func (s *SafetyControls) UnmarshalJSON(data []byte) error {
	type plain SafetyControls
	shadow := struct {
		*plain
		MaxCPUPct *jsonint.Int `json:"maxCpuPct,omitempty"`
		MaxMemMB  *jsonint.Int `json:"maxMemMB,omitempty"`
	}{plain: (*plain)(s), MaxCPUPct: (*jsonint.Int)(s.MaxCPUPct), MaxMemMB: (*jsonint.Int)(s.MaxMemMB)}
	if err := json.Unmarshal(data, &shadow); err != nil {
		return err
	}
	s.MaxCPUPct, s.MaxMemMB = (*int)(shadow.MaxCPUPct), (*int)(shadow.MaxMemMB)
	return nil
}

// LocalizedMetadata holds translated strings for one locale key.
type LocalizedMetadata struct {
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	CreditLine  string `json:"creditLine,omitempty"`
}
