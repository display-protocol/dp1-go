package dp1

import (
	"encoding/json"
	"fmt"

	"github.com/display-protocol/dp1-go/extension/channels"
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
	var p playlist.Playlist
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, WithCode(CodePlaylistInvalid, fmt.Errorf("dp1: decode playlist: %w", err))
	}
	return &p, nil
}

// ParseAndValidatePlaylistWithPlaylistsExtension validates against the composed playlists extension schema (core bundle + extension fragment).
func ParseAndValidatePlaylistWithPlaylistsExtension(data []byte) (*playlist.Playlist, error) {
	if err := PlaylistWithPlaylistsExtensionSchemaValidate(data); err != nil {
		return nil, CodeFromPlaylistValidation(err)
	}
	var p playlist.Playlist
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, WithCode(CodePlaylistInvalid, fmt.Errorf("dp1: decode playlist: %w", err))
	}
	return &p, nil
}

// ParseAndValidatePlaylistWithContentRatingExtension validates core plus the draft content-rating extension.
func ParseAndValidatePlaylistWithContentRatingExtension(data []byte) (*playlist.Playlist, error) {
	if err := PlaylistWithContentRatingExtensionSchemaValidate(data); err != nil {
		return nil, CodeFromPlaylistValidation(err)
	}
	var p playlist.Playlist
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, WithCode(CodePlaylistInvalid, fmt.Errorf("dp1: decode playlist: %w", err))
	}
	return &p, nil
}

// ParseAndValidatePlaylistWithPlaylistsAndContentRatingExtensions validates both draft overlays.
func ParseAndValidatePlaylistWithPlaylistsAndContentRatingExtensions(data []byte) (*playlist.Playlist, error) {
	if err := PlaylistWithPlaylistsAndContentRatingExtensionsSchemaValidate(data); err != nil {
		return nil, CodeFromPlaylistValidation(err)
	}
	var p playlist.Playlist
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, WithCode(CodePlaylistInvalid, fmt.Errorf("dp1: decode playlist: %w", err))
	}
	return &p, nil
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
