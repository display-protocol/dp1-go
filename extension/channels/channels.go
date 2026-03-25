// Package channels contains types for the DP-1 "channels" extension (playlist-group evolution).
package channels

import (
	"github.com/display-protocol/dp1-go/extension/identity"
	"github.com/display-protocol/dp1-go/playlist"
)

// Channel is a collaborative feed document extending the playlist-group model.
type Channel struct {
	ID         string               `json:"id"`
	Slug       string               `json:"slug"`
	Title      string               `json:"title"`
	Version    string               `json:"version"`
	Created    string               `json:"created"`
	Playlists  []string             `json:"playlists"`
	Curators   []identity.Entity    `json:"curators,omitempty"`
	Publisher  *identity.Entity     `json:"publisher,omitempty"`
	Summary    string               `json:"summary,omitempty"`
	CoverImage string               `json:"coverImage,omitempty"`
	Signatures []playlist.Signature `json:"signatures,omitempty"`
	Signature  string               `json:"signature,omitempty"`
}
