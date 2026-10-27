// Package playlists contains types for the DP-1 "playlists" extension (draft): optional fields on a playlist document.
package playlists

import (
	"encoding/json"

	"github.com/display-protocol/dp1-go/extension/identity"
)

// Note is an experimental intermission card: short artist-authored text before the playlist or an item (not DP-1 core).
type Note struct {
	Text     string   `json:"text"`
	Duration *float64 `json:"duration,omitempty"` // display seconds when present; schema default 20 when omitted
	// Title is the heading shown above the text (dp1#55).
	Title string `json:"title,omitempty"`
	// Author is who wrote the text; omitted, the note is the curators' own (dp1#55).
	Author *identity.Entity `json:"author,omitempty"`
}

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
	Note     *Note             `json:"note,omitempty"`
	OpenAt   string            `json:"openAt,omitempty"` // see playlist.Playlist.OpenAt (dp1#55)
	Curators []identity.Entity `json:"curators,omitempty"`
	Summary  string            `json:"summary,omitempty"`
	// Links is editorial content about the playlist (articles, events, news); see
	// playlist.Playlist.Links (dp1#55).
	Links        []identity.Link `json:"links,omitempty"`
	CoverImage   string          `json:"coverImage,omitempty"`
	DynamicQuery *DynamicQuery   `json:"dynamicQuery,omitempty"`
	// Series is the series the playlist's items belong to; see playlist.Playlist.Series
	// (dp1#55). Raw JSON here rather than []playlist.SeriesEntry: an entry's provenance is a
	// core playlist type, and this package cannot import playlist without a cycle.
	Series json.RawMessage `json:"series,omitempty"`
}
