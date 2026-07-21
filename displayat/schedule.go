package displayat

import (
	"time"

	"github.com/display-protocol/dp1-go/playlist"
)

// ComputeActiveSet returns the items that should be played at the given time
// when schedule.byDisplayAt is true. See DP-1 Playlist Extension §3.5.3.
//
// Logic:
//  1. Filter items where displayAt <= now (resolved instant comparison).
//  2. Find max displayAt instant among filtered items.
//  3. Return items with displayAt == max instant, plus items without displayAt (evergreen).
//  4. Preserve original order.
//
// If all timed items are in the future, returns only evergreen items.
// If no items qualify, returns an empty slice.
//
// The loc parameter is used to resolve date-only and local datetime displayAt values
// (pass the playback device's local timezone).
func ComputeActiveSet(p *playlist.Playlist, now time.Time, loc *time.Location) []playlist.PlaylistItem {
	if loc == nil {
		loc = time.UTC
	}

	type resolved struct {
		item    playlist.PlaylistItem
		parsed  Parsed
		instant time.Time // resolved instant; zero if no displayAt (evergreen)
		isTimed bool
	}

	items := make([]resolved, 0, len(p.Items))
	for _, it := range p.Items {
		r := resolved{item: it}
		if it.DisplayAt != "" {
			r.parsed = Parse(it.DisplayAt, loc)
			if r.parsed.IsValid() {
				r.instant = r.parsed.Resolved
				r.isTimed = true
			}
		}
		items = append(items, r)
	}

	// Find max instant among items with displayAt <= now.
	var maxInstant time.Time
	hasPast := false
	for _, r := range items {
		if r.isTimed && !r.instant.After(now) {
			if !hasPast || r.instant.After(maxInstant) {
				maxInstant = r.instant
				hasPast = true
			}
		}
	}

	// Build active set: timed cohort (displayAt == maxInstant) + evergreen (no displayAt).
	result := make([]playlist.PlaylistItem, 0)
	for _, r := range items {
		if r.isTimed {
			if hasPast && r.instant.Equal(maxInstant) {
				result = append(result, r.item)
			}
		} else {
			result = append(result, r.item)
		}
	}

	return result
}

// NextDisplayAt returns the smallest displayAt instant that is strictly after now,
// or nil if there are no future displayAt values.
//
// The loc parameter is used to resolve date-only and local datetime displayAt values
// (pass the playback device's local timezone).
func NextDisplayAt(p *playlist.Playlist, now time.Time, loc *time.Location) *time.Time {
	if loc == nil {
		loc = time.UTC
	}

	var next *time.Time
	for _, it := range p.Items {
		if it.DisplayAt == "" {
			continue
		}
		parsed := Parse(it.DisplayAt, loc)
		if !parsed.IsValid() {
			continue
		}
		instant := parsed.Resolved
		if instant.After(now) {
			if next == nil || instant.Before(*next) {
				t := instant
				next = &t
			}
		}
	}

	return next
}
