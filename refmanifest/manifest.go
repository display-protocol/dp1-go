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

// Artist identifies a creator and, from refVersion 1.1.0, carries a
// profile snapshot taken when the manifest was authored (spec §4.1).
//
// Identity: only Addresses may be used to recognize the same artist across
// producers. ID is producer-scoped (two producers label one artist with
// different ids), and Name is display text. Addresses is a signed claim of
// the producer, not a fact — a wallet can be shared by a collective or a
// collaborative mint — so consumers must not merge two artist records merely
// because their address lists intersect.
//
// Profile fields (Avatar, Biographies, Links) are the offline fallback, not
// the source of truth; a consumer may overlay fresher registry data keyed by
// Addresses.
type Artist struct {
	Name string `json:"name"`
	// ID is an opaque, producer-scoped label. Not an identity.
	ID string `json:"id,omitempty"`
	// URL is the refVersion 1.0.0 single profile URL.
	//
	// Deprecated: superseded by Links (write an entry of type "website").
	// Still valid on the wire; a producer that emits both must keep URL equal
	// to that entry. Consumers read Links first and fall back to URL only
	// when Links is absent or empty.
	URL string `json:"url,omitempty"`
	// Addresses holds raw wallet addresses (EVM 0x…, Tezos tz1…tz4). Contract
	// addresses (Tezos KT1…, EVM collection contracts) name a collection, not
	// a person, and must not be listed — they belong in provenance.contract.
	Addresses []string `json:"addresses,omitempty"`
	// Avatar is a portrait or profile image; same shape as a thumbnail.
	Avatar *Thumbnail `json:"avatar,omitempty"`
	// Biographies are ordered by the producer's preference; when only one
	// fits, show the first.
	Biographies []Biography `json:"biographies,omitempty"`
	// Links are typed profile links with full URLs, never bare handles.
	Links []Link `json:"links,omitempty"`
}

// Biography is one biographical text with optional attribution.
type Biography struct {
	// Text is plain text, no markup.
	Text string `json:"text"`
	// Source names the publication or platform the text comes from.
	Source string `json:"source,omitempty"`
	// SourceURL is the URL of that source.
	SourceURL string `json:"sourceUrl,omitempty"`
}

// Link is an external profile link.
type Link struct {
	// Type is one of the LinkType* constants; the schema rejects anything else.
	Type string `json:"type"`
	// URL is always a full URL, never a bare handle, so players carry no
	// per-network URL rules.
	URL string `json:"url"`
}

// Link types accepted by the schema's links[].type enumeration. A destination
// the enumeration does not name is written as LinkTypeOther.
const (
	LinkTypeWebsite   = "website"
	LinkTypeTwitter   = "twitter"
	LinkTypeInstagram = "instagram"
	LinkTypeOther     = "other"
)

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
