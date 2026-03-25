// Package playlists contains types for the DP-1 "playlists" extension (draft): optional fields on a playlist document.
package playlists

import "github.com/display-protocol/dp1-go/extension/identity"

// DynamicQuery configures dynamic item fetching from an indexer.
type DynamicQuery struct {
	Profile         string            `json:"profile"`
	Endpoint        string            `json:"endpoint"`
	Method          string            `json:"method,omitempty"`
	Headers         map[string]string `json:"headers,omitempty"`
	Query           string            `json:"query,omitempty"`
	ResponseMapping ResponseMapping   `json:"responseMapping"`
}

// ResponseMapping describes how to map indexer JSON to playlist items.
type ResponseMapping struct {
	ItemsPath  string            `json:"itemsPath"`
	ItemSchema string            `json:"itemSchema"`
	ItemMap    map[string]string `json:"itemMap,omitempty"`
}

// Overlay is the optional top-level extension object merged into a core playlist JSON value.
type Overlay struct {
	Curators     []identity.Entity `json:"curators,omitempty"`
	Summary      string            `json:"summary,omitempty"`
	CoverImage   string            `json:"coverImage,omitempty"`
	DynamicQuery *DynamicQuery     `json:"dynamicQuery,omitempty"`
}
