// Package playlists contains types for the DP-1 "playlists" extension (draft): optional fields on a playlist document.
package playlists

import "github.com/display-protocol/dp1-go/extension/identity"

// Note is an experimental intermission card: short artist-authored text before the playlist or an item (not DP-1 core).
type Note struct {
	Text     string   `json:"text"`
	Duration *float64 `json:"duration,omitempty"` // display seconds when present; schema default 20 when omitted
	// Title is a heading shown above Text (1–200 characters).
	Title string `json:"title,omitempty"`
	// Author is who wrote Text. Nil means the note is the curators' own.
	Author *identity.Entity `json:"author,omitempty"`
}

// Satellite names a smaller playlist published around this one — a burn record, a set of
// mementos, a catalog or launch-party edition — that a consumer implementing this section
// SHOULD present as a labeled section of this playlist's page rather than as a show of its
// own (§3.8). The satellite's own playlist remains a complete, independently valid,
// independently signable Playlist; this entry only names it and how to present it, and the
// satellite's own document carries no back-reference to its parent (dp1#63).
type Satellite struct {
	// Playlist is the absolute URI of the satellite's own playlist. Not a channel-relative
	// or otherwise relative address: the schema's format:uri already requires an absolute
	// URI, and this field defines no relative-resolution rule.
	Playlist string `json:"playlist"`
	// Kind is open vocabulary (as in content-rating): e.g. "burn",
	// "mementos", "catalog", "launch-party". Any string is valid; an unrecognized value is
	// still a valid satellite, just an unlabeled one.
	Kind string `json:"kind"`
	// Title is a heading shown above the satellite's items (1-200 characters), when
	// different from a human-readable form of Kind.
	Title string `json:"title,omitempty"`
	// Note is one paragraph of context shown above the satellite's items (1-2000
	// characters), e.g. what a memento is and when it was given out.
	Note string `json:"note,omitempty"`
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
	// Visibility is the listing request; nil is absent (public). See playlist.Playlist.Visibility (§3.7).
	Visibility *string `json:"visibility,omitempty"`
	// Satellites names smaller playlists published around this one, rendered as labeled
	// sections of this playlist's page. See playlist.Playlist.Satellites (§3.8).
	Satellites []Satellite `json:"satellites,omitempty"`
}
