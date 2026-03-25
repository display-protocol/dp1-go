// Package identity holds shared types used by multiple DP-1 registry extensions.
package identity

// Entity is a human-facing name plus a verifiable DID key (and optional URL), as used by
// the channels and playlists extensions.
type Entity struct {
	Name string `json:"name"`
	Key  string `json:"key"`
	URL  string `json:"url,omitempty"`
}
