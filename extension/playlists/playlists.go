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

// UnmarshalJSON decodes a Satellite, taking every member by its exact JSON key instead of
// trusting encoding/json's own field matching for any of them.
//
// The playlists-extension schema's Satellite definition permits properties it does not
// describe, so a document carrying both "playlist" and an unrecognized "Playlist" (or any other
// case variant of playlist/kind/title/note) validates — the second is merely an unknown
// property, of any JSON type. encoding/json's case-insensitive fallback would nonetheless let
// it silently overwrite the validated value (or, for a non-string value, fail the decode
// outright), the same risk PlaylistItem's content-rating fields avoid (playlist/playlist.go) by
// exact-key lookup. Unlike Playlist, Satellite is a leaf type with no known embedder, so giving
// it its own UnmarshalJSON carries none of the method-promotion risk documented on
// Playlist.Visibility.
//
// Every value comes from a map[string]json.RawMessage decode of data, never from a plain
// json.Unmarshal into a typed struct. A typed decode — even used only to validate shape, with
// its field values discarded — would still run encoding/json's own case-insensitive field
// matching as a side effect: a document carrying only a wrong-case, wrongly-typed key (no
// exact-case counterpart at all, e.g. {"playlist":"...","kind":"...","Title":5}, a number) would
// have that value matched onto the typed Title string field and fail the whole decode with a
// type error — rejecting a document the schema accepts (it has no additionalProperties:false),
// and, one level up, making decodeSatellitesExact's shape tolerance (parse.go) silently drop
// the entire satellites array for a document this method was never meant to reject at all.
// Decoding into the map sidesteps struct field matching entirely: a map key is a literal string,
// and only the four exact keys below are ever read.
func (s *Satellite) UnmarshalJSON(data []byte) error {
	var members map[string]json.RawMessage
	if err := json.Unmarshal(data, &members); err != nil {
		return err
	}
	*s = Satellite{}
	for key, field := range map[string]*string{
		"playlist": &s.Playlist,
		"kind":     &s.Kind,
		"title":    &s.Title,
		"note":     &s.Note,
	} {
		raw, ok := members[key]
		if !ok {
			continue
		}
		var v string
		if err := json.Unmarshal(raw, &v); err != nil {
			return err
		}
		*field = v
	}
	return nil
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
