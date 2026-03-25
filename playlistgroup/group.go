// Package playlistgroup defines DP-1 playlist-group (exhibition) documents.
package playlistgroup

import "github.com/display-protocol/dp1-go/playlist"

// Group is a curator-authored collection of playlist URIs (DP-1 §15.2).
type Group struct {
	ID         string               `json:"id"`
	Slug       string               `json:"slug,omitempty"`
	Title      string               `json:"title"`
	Curator    string               `json:"curator,omitempty"`
	Summary    string               `json:"summary,omitempty"`
	Playlists  []string             `json:"playlists"`
	Created    string               `json:"created"`
	CoverImage string               `json:"coverImage,omitempty"`
	Signatures []playlist.Signature `json:"signatures,omitempty"`
	Signature  string               `json:"signature,omitempty"`
}
