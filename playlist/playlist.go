// Package playlist defines DP-1 major-1 playlist document types (core wire format).
package playlist

import (
	"encoding/json"

	"github.com/display-protocol/dp1-go/extension/identity"
	"github.com/display-protocol/dp1-go/extension/playlists"
)

// Playlist is a DP-1 playlist (v1.0.x legacy signature and/or v1.1+ multi-signature).
type Playlist struct {
	DPVersion  string         `json:"dpVersion"`
	ID         string         `json:"id,omitempty"`
	Title      string         `json:"title"`
	Slug       string         `json:"slug,omitempty"`
	Created    string         `json:"created,omitempty"`
	Note       *Note          `json:"note,omitempty"`
	Defaults   *Defaults      `json:"defaults,omitempty"`
	Items      []PlaylistItem `json:"items"`
	Signatures []Signature    `json:"signatures,omitempty"` // v1.1+
	Signature  string         `json:"signature,omitempty"`  // legacy v1.0.x (deprecated)

	// --- Registry extension: "playlists" (draft) — additive optional fields only ---
	// These are not part of DP-1 core JSON Schema; they are validated only when using
	// ParseAndValidatePlaylistWithPlaylistsExtension. Safe to omit for core-only documents.
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

// Note is optional explanatory metadata shown by supporting players before a playlist or item.
type Note struct {
	Text            string `json:"text"`
	DisplayDuration *int   `json:"display_duration,omitempty"`
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
	Note       *Note            `json:"note,omitempty"`
	Override   json.RawMessage  `json:"override,omitempty"`
	Display    *DisplayPrefs    `json:"display,omitempty"`
	Repro      *ReproBlock      `json:"repro,omitempty"`
	Provenance *ProvenanceBlock `json:"provenance,omitempty"`
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
