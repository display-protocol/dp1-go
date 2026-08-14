// Package playlist defines DP-1 major-1 playlist document types (core wire format).
package playlist

import (
	"encoding/json"
	"fmt"

	"github.com/display-protocol/dp1-go/extension/identity"
	"github.com/display-protocol/dp1-go/extension/playlists"
	"github.com/display-protocol/dp1-go/refmanifest"
)

// Playlist is a DP-1 playlist (v1.0.x legacy signature and/or v1.1+ multi-signature).
type Playlist struct {
	DPVersion  string         `json:"dpVersion"`
	ID         string         `json:"id,omitempty"`
	Title      string         `json:"title"`
	Slug       string         `json:"slug,omitempty"`
	Created    string         `json:"created,omitempty"`
	Defaults   *Defaults      `json:"defaults,omitempty"`
	Items      []PlaylistItem `json:"items"`
	Signatures []Signature    `json:"signatures,omitempty"` // v1.1+
	Signature  string         `json:"signature,omitempty"`  // legacy v1.0.x (deprecated)

	// --- Registry extension: "playlists" (draft) — additive optional fields only ---
	// These are not part of DP-1 core JSON Schema; they are validated only when using
	// ParseAndValidatePlaylistWithPlaylistsExtension. Safe to omit for core-only documents.
	Note         *playlists.Note         `json:"note,omitempty"`
	Curators     []identity.Entity       `json:"curators,omitempty"`
	Summary      string                  `json:"summary,omitempty"`
	CoverImage   string                  `json:"coverImage,omitempty"`
	DynamicQuery *playlists.DynamicQuery `json:"dynamicQuery,omitempty"`
}

// Defaults holds playlist-level defaults inherited by items.
type Defaults struct {
	Display  *DisplayPrefs `json:"display,omitempty"`
	License  string        `json:"license,omitempty"`
	Duration *float64      `json:"duration,omitempty"`
}

// PlaylistItem is one entry in a playlist.
type PlaylistItem struct {
	ID         string           `json:"id,omitempty"`
	Slug       string           `json:"slug,omitempty"`
	Title      string           `json:"title,omitempty"`
	Source     string           `json:"source"`
	Duration   *float64         `json:"duration,omitempty"`
	License    string           `json:"license,omitempty"`
	Ref        string           `json:"ref,omitempty"`
	RefHash    string           `json:"refHash,omitempty"` // prose spec; not in core JSON Schema yet
	Override   json.RawMessage  `json:"override,omitempty"`
	Display    *DisplayPrefs    `json:"display,omitempty"`
	Repro      *ReproBlock      `json:"repro,omitempty"`
	Provenance *ProvenanceBlock `json:"provenance,omitempty"`

	// Playlists extension only (ParseAndValidatePlaylistWithPlaylistsExtension); not in core schema.
	Note *playlists.Note `json:"note,omitempty"`
	// DisplayAt is an optional ISO 8601 scheduling datetime (§3.5.2; date-only not accepted).
	// Nil means the field is absent (evergreen when displayAt scheduling is active). A non-nil pointer
	// (including to "") means the field is present; unresolvable values are not eligible (§3.5.5).
	DisplayAt *string `json:"displayAt,omitempty"`
	// InlineManifest carries a complete Ref Manifest inline instead of behind Ref (§3.6):
	// the same document, checked by the unmodified ref-manifest schema, so a malformed one
	// invalidates the playlist on the playlists-extension path.
	//
	// Held as raw JSON, like Override, rather than as a decoded manifest. The core schema does
	// not describe this field and core DP-1 tolerates unknown ones, so a core-only player must
	// be able to parse a playlist carrying an inlineManifest it does not implement — including
	// one whose shape it would reject. A typed field would fail the decode step of
	// ParseAndValidatePlaylist on documents the core schema accepted. Keeping the bytes also
	// keeps them verbatim: they are covered by the playlist signature (core §7.1) with no
	// refHash counterpart, and re-encoding a decoded manifest would drop present-but-empty
	// fields (the artist "id": "" in the §3.6 example) and change the JCS payload.
	//
	// Use ParseInlineManifest to decode, or hand these bytes straight to
	// dp1.ParseAndValidateRefManifest for decode plus full schema validation.
	//
	// Precedence when both are present: defaults → inlineManifest → ref → item-local, i.e. a
	// fetched Ref manifest wins and the inline copy is the offline/degraded fallback.
	InlineManifest json.RawMessage `json:"inlineManifest,omitempty"`
}

// ParseInlineManifest decodes the item's inline Ref Manifest (§3.6).
//
// Returns (nil, nil) when the field is absent or JSON null, so callers can branch on the
// manifest alone. Decode only — the bytes are schema-checked when the playlist was parsed with
// dp1.ParseAndValidatePlaylistWithPlaylistsExtension; on the core-only path they are whatever
// the document carried, and an error here is how that surfaces. For decode plus validation,
// pass item.InlineManifest to dp1.ParseAndValidateRefManifest instead.
func (it PlaylistItem) ParseInlineManifest() (*refmanifest.Manifest, error) {
	if len(it.InlineManifest) == 0 || string(it.InlineManifest) == "null" {
		return nil, nil
	}
	var m refmanifest.Manifest
	if err := json.Unmarshal(it.InlineManifest, &m); err != nil {
		return nil, fmt.Errorf("playlist: decode inlineManifest: %w", err)
	}
	return &m, nil
}

// DisplayPrefs controls how a player renders an item (see DP-1 §4).
type DisplayPrefs struct {
	Scaling       string            `json:"scaling,omitempty"`
	Margin        json.RawMessage   `json:"margin,omitempty"` // number or "%|vw|vh" string
	Background    string            `json:"background,omitempty"`
	Autoplay      *bool             `json:"autoplay,omitempty"`
	Loop          *bool             `json:"loop,omitempty"`
	Interaction   *InteractionPrefs `json:"interaction,omitempty"`
	UserOverrides map[string]bool   `json:"userOverrides,omitempty"`
}

// InteractionPrefs configures input (keyboard / mouse).
type InteractionPrefs struct {
	Keyboard []string    `json:"keyboard,omitempty"`
	Mouse    *MousePrefs `json:"mouse,omitempty"`
}

// MousePrefs toggles pointer interactions.
//
// These are plain bools, so an explicit false is indistinguishable from an absent key and the
// merge order cannot express a revocation: see #6, which changes them to pointers. Left as they
// are here to keep this change to the two spec updates it implements.
type MousePrefs struct {
	Click  bool `json:"click,omitempty"`
	Scroll bool `json:"scroll,omitempty"`
	Drag   bool `json:"drag,omitempty"`
	Hover  bool `json:"hover,omitempty"`
}

// ReproBlock carries deterministic reproduction hints (DP-1 §5).
type ReproBlock struct {
	EngineVersion map[string]string `json:"engineVersion,omitempty"`
	Seed          string            `json:"seed,omitempty"`
	AssetsSHA256  []string          `json:"assetsSHA256,omitempty"`
	FrameHash     *FrameHash        `json:"frameHash,omitempty"`
}

// FrameHash holds first-frame verification hashes.
type FrameHash struct {
	SHA256 string `json:"sha256,omitempty"`
	Phash  string `json:"phash,omitempty"`
}

// ProvenanceType identifies how provenance is expressed.
type ProvenanceType string

const (
	ProvenanceOnChain        ProvenanceType = "onChain"
	ProvenanceSeriesRegistry ProvenanceType = "seriesRegistry"
	ProvenanceOffChainURI    ProvenanceType = "offChainURI"
)

// ProvenanceBlock links rendered assets to chain or off-chain records (DP-1 §6).
type ProvenanceBlock struct {
	Type         ProvenanceType      `json:"type"`
	Contract     *ProvenanceContract `json:"contract,omitempty"`
	Dependencies []ProvenanceDep     `json:"dependencies,omitempty"`
}

// ProvenanceContract is contract-specific provenance data.
type ProvenanceContract struct {
	Chain    string `json:"chain,omitempty"`
	Standard string `json:"standard,omitempty"`
	Address  string `json:"address,omitempty"`
	SeriesID *int   `json:"seriesId,omitempty"`
	TokenID  string `json:"tokenId,omitempty"`
	URI      string `json:"uri,omitempty"`
	MetaHash string `json:"metaHash,omitempty"`
}

// ProvenanceDep is documentary only; players must not fetch at runtime.
type ProvenanceDep struct {
	Chain    string `json:"chain,omitempty"`
	Standard string `json:"standard,omitempty"`
	URI      string `json:"uri,omitempty"`
}

// Signature is one entry in a v1.1+ signature chain (DP-1 §7.1.1).
type Signature struct {
	Alg         string `json:"alg"`
	Kid         string `json:"kid"`
	Ts          string `json:"ts"`
	PayloadHash string `json:"payload_hash"`
	Role        string `json:"role"`
	Sig         string `json:"sig"`
}

// Signature roles (DP-1 §7.1.1).
const (
	RoleCurator     = "curator"
	RoleFeed        = "feed"
	RoleAgent       = "agent"
	RoleInstitution = "institution"
	RoleLicensor    = "licensor"
)

// Algorithm values from the core schema (more may appear in future spec minors).
const (
	AlgEd25519        = "ed25519"
	AlgEIP191         = "eip191"
	AlgECDSASecp256k1 = "ecdsa-secp256k1"
	AlgECDSAP256      = "ecdsa-p256"
)
