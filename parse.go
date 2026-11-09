package dp1

import (
	"encoding/json"
	"fmt"

	"github.com/display-protocol/dp1-go/extension/channels"
	"github.com/display-protocol/dp1-go/extension/playlists"
	"github.com/display-protocol/dp1-go/internal/validate"
	"github.com/display-protocol/dp1-go/playlist"
	"github.com/display-protocol/dp1-go/playlistgroup"
	"github.com/display-protocol/dp1-go/refmanifest"
)

// The following hooks default to the real JSON Schema validators. Tests may replace them briefly
// to exercise JSON decode error paths. Do not reassign concurrently in production.
var (
	PlaylistCoreSchemaValidate                                    = validate.Playlist
	PlaylistWithPlaylistsExtensionSchemaValidate                  = validate.PlaylistWithPlaylistsExtension
	PlaylistWithContentRatingExtensionSchemaValidate              = validate.PlaylistWithContentRatingExtension
	PlaylistWithPlaylistsAndContentRatingExtensionsSchemaValidate = validate.PlaylistWithPlaylistsAndContentRatingExtensions
	PlaylistGroupSchemaValidate                                   = validate.PlaylistGroup
	RefManifestSchemaValidate                                     = validate.RefManifest
	ChannelExtensionSchemaValidate                                = validate.ChannelsExtension
)

// decodePlaylist decodes a playlist document, then corrects Playlist.Satellites against the raw
// bytes rather than trusting the struct decode above for that one field.
//
// satellites is schema-validated only on the playlists-extension-aware paths — the core and
// content-rating schemas describe no such property — so, like InlineManifest and ContentRating,
// the Go field for it has to tolerate a document those other paths accept but whose "satellites"
// member does not fit []playlists.Satellite (e.g. a number): a plain typed field would fail the
// whole decode on a document the schema had just accepted. The shadow struct below absorbs
// whatever key matches "satellites" as raw bytes, so the json.Unmarshal that populates every
// other field can never fail because of it.
//
// It also has to be read back by its exact JSON key. encoding/json falls back to a
// case-insensitive match when no field claims a key exactly, and claims the field by whichever
// key is processed last in document order: since the shadow field is itself tagged "satellites",
// a document carrying both "satellites" and an unrecognized "Satellites" (or any other case
// variant) would let the second silently overwrite the raw bytes meant for the first — one that
// passed schema validation, the other that the schema sees only as an unknown property. Pulling
// the exact key from a decoded map[string]json.RawMessage sidesteps that fallback entirely:
// map keys are literal strings, never folded.
//
// This runs as a second pass over data instead of a Playlist.UnmarshalJSON, which
// playlist.Playlist deliberately does not have — see the comment on Playlist.Visibility for why
// one would be promoted into structs that embed Playlist and take over their decoding.
func decodePlaylist(data []byte) (*playlist.Playlist, error) {
	type shadow playlist.Playlist
	var s struct {
		shadow
		Satellites json.RawMessage `json:"satellites,omitempty"`
	}
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	p := playlist.Playlist(s.shadow)
	p.Satellites = decodeSatellitesExact(data)
	return &p, nil
}

// decodeSatellitesExact extracts "satellites" by its exact JSON key and decodes it leniently:
// a value of the wrong JSON shape (tolerated on paths that do not validate it) decodes as
// absent rather than failing, the same "left nil" tolerance ContentRating relies on.
func decodeSatellitesExact(data []byte) []playlists.Satellite {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(data, &top); err != nil {
		return nil
	}
	raw, ok := top["satellites"]
	if !ok || string(raw) == "null" {
		return nil
	}
	var satellites []playlists.Satellite
	if err := json.Unmarshal(raw, &satellites); err != nil {
		return nil
	}
	return satellites
}

// ValidateContentRatingExtension validates only the draft content-rating overlay.
// It accepts a partial/full playlist without requiring core fields or signatures, making it
// suitable for legacy ingestion boundaries that cannot yet require a signed DP-1 document.
func ValidateContentRatingExtension(data []byte) error {
	if err := validate.ContentRatingExtensionFragment(data); err != nil {
		return CodeFromPlaylistValidation(err)
	}
	return nil
}

// ParseAndValidatePlaylist validates against the core playlist schema and decodes into playlist.Playlist.
//
// Only core fields are guaranteed schema-valid: the core schema describes no registry-extension
// field, so an inlineManifest present in the document decodes without ever being checked, and
// the merge package feeds its controls straight into display preferences. Callers that act on
// it should parse with [ParseAndValidatePlaylistWithPlaylistsExtension] instead.
func ParseAndValidatePlaylist(data []byte) (*playlist.Playlist, error) {
	if err := PlaylistCoreSchemaValidate(data); err != nil {
		return nil, CodeFromPlaylistValidation(err)
	}
	p, err := decodePlaylist(data)
	if err != nil {
		return nil, WithCode(CodePlaylistInvalid, fmt.Errorf("dp1: decode playlist: %w", err))
	}
	return p, nil
}

// ParseAndValidatePlaylistWithPlaylistsExtension validates against the composed playlists extension schema (core bundle + extension fragment).
func ParseAndValidatePlaylistWithPlaylistsExtension(data []byte) (*playlist.Playlist, error) {
	if err := PlaylistWithPlaylistsExtensionSchemaValidate(data); err != nil {
		return nil, CodeFromPlaylistValidation(err)
	}
	p, err := decodePlaylist(data)
	if err != nil {
		return nil, WithCode(CodePlaylistInvalid, fmt.Errorf("dp1: decode playlist: %w", err))
	}
	return p, nil
}

// ParseAndValidatePlaylistWithContentRatingExtension validates core plus the draft content-rating extension.
func ParseAndValidatePlaylistWithContentRatingExtension(data []byte) (*playlist.Playlist, error) {
	if err := PlaylistWithContentRatingExtensionSchemaValidate(data); err != nil {
		return nil, CodeFromPlaylistValidation(err)
	}
	p, err := decodePlaylist(data)
	if err != nil {
		return nil, WithCode(CodePlaylistInvalid, fmt.Errorf("dp1: decode playlist: %w", err))
	}
	return p, nil
}

// ParseAndValidatePlaylistWithPlaylistsAndContentRatingExtensions validates both draft overlays.
func ParseAndValidatePlaylistWithPlaylistsAndContentRatingExtensions(data []byte) (*playlist.Playlist, error) {
	if err := PlaylistWithPlaylistsAndContentRatingExtensionsSchemaValidate(data); err != nil {
		return nil, CodeFromPlaylistValidation(err)
	}
	p, err := decodePlaylist(data)
	if err != nil {
		return nil, WithCode(CodePlaylistInvalid, fmt.Errorf("dp1: decode playlist: %w", err))
	}
	return p, nil
}

// ParseAndValidatePlaylistGroup validates a playlist-group (exhibition) document.
func ParseAndValidatePlaylistGroup(data []byte) (*playlistgroup.Group, error) {
	if err := PlaylistGroupSchemaValidate(data); err != nil {
		return nil, CodeFromPlaylistGroupValidation(err)
	}
	var g playlistgroup.Group
	if err := json.Unmarshal(data, &g); err != nil {
		return nil, WithCode(CodePlaylistGroupInvalid, fmt.Errorf("dp1: decode playlist-group: %w", err))
	}
	return &g, nil
}

// ParseAndValidateRefManifest validates a ref manifest document.
func ParseAndValidateRefManifest(data []byte) (*refmanifest.Manifest, error) {
	if err := RefManifestSchemaValidate(data); err != nil {
		return nil, CodeFromRefManifestValidation(err)
	}
	var m refmanifest.Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, WithCode(CodeRefManifestInvalid, fmt.Errorf("dp1: decode ref manifest: %w", err))
	}
	return &m, nil
}

// ParseAndValidateChannel validates the channels extension document.
func ParseAndValidateChannel(data []byte) (*channels.Channel, error) {
	if err := ChannelExtensionSchemaValidate(data); err != nil {
		return nil, CodeFromChannelValidation(err)
	}
	var ch channels.Channel
	if err := json.Unmarshal(data, &ch); err != nil {
		return nil, WithCode(CodeChannelInvalid, fmt.Errorf("dp1: decode channel: %w", err))
	}
	return &ch, nil
}
