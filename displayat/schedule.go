package displayat

import (
	"time"

	"github.com/display-protocol/dp1-go/playlist"
)

// ComputeActiveSet returns the items that should be played at the given time.
// See DP-1 Playlist Extension §3.5.1 / §3.5.3 / §3.5.5.
//
// When no item has displayAt, returns a copy of all items and ignores displayAt
// for filtering (§3.5.1).
//
// When any item has displayAt, displayAt scheduling is active:
//  1. Resolve each item’s displayAt to an instant (§3.5.2).
//  2. Find max displayAt instant among items with resolvable displayAt ≤ now.
//  3. Return items with displayAt == max instant, plus items with no displayAt field (evergreen).
//  4. Preserve original order.
//
// An item whose displayAt field is present but cannot be resolved (invalid calendar/clock
// after accepting the wire pattern) is excluded from the active set and is not evergreen
// (§3.5.5). NextDisplayAt likewise ignores unresolvable values as timer candidates.
//
// If all timed items are in the future, returns only evergreen items.
// If no items qualify, returns an empty slice.
//
// The loc parameter is used to resolve local datetime displayAt values
// (pass the display-locale timezone).
func ComputeActiveSet(p *playlist.Playlist, now time.Time, loc *time.Location) []playlist.PlaylistItem {
	if p == nil {
		return nil
	}
	// §3.5.1: filtering activates automatically when at least one item has displayAt.
	if !hasDisplayAt(p.Items) {
		out := make([]playlist.PlaylistItem, len(p.Items))
		copy(out, p.Items)
		return out
	}
	if loc == nil {
		loc = time.UTC
	}

	type resolved struct {
		item      playlist.PlaylistItem
		parsed    Parsed
		instant   time.Time // resolved instant when isTimed
		isTimed   bool      // displayAt present and resolvable
		evergreen bool      // displayAt field absent
	}

	items := make([]resolved, 0, len(p.Items))
	for _, it := range p.Items {
		r := resolved{item: it}
		if it.DisplayAt == nil {
			r.evergreen = true
		} else {
			r.parsed = Parse(*it.DisplayAt, loc)
			if r.parsed.IsValid() {
				r.instant = r.parsed.Resolved
				r.isTimed = true
			}
			// Present but unresolvable (including ""): neither timed nor evergreen (§3.5.5).
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

	// Build active set: timed cohort (displayAt == maxInstant) + evergreen (no displayAt field).
	result := make([]playlist.PlaylistItem, 0)
	for _, r := range items {
		switch {
		case r.isTimed:
			if hasPast && r.instant.Equal(maxInstant) {
				result = append(result, r.item)
			}
		case r.evergreen:
			result = append(result, r.item)
		}
	}

	return result
}

// NextDisplayAt returns the smallest displayAt instant that is strictly after now,
// or nil if there are no future displayAt values.
//
// When no item has displayAt, returns nil (§3.5.1; scheduling is inactive).
//
// Unresolvable displayAt values are skipped (they are not timer candidates per §3.5.5).
//
// The loc parameter is used to resolve local datetime displayAt values
// (pass the display-locale timezone).
func NextDisplayAt(p *playlist.Playlist, now time.Time, loc *time.Location) *time.Time {
	if p == nil {
		return nil
	}
	if !hasDisplayAt(p.Items) {
		return nil
	}
	if loc == nil {
		loc = time.UTC
	}

	var next *time.Time
	for _, it := range p.Items {
		if it.DisplayAt == nil {
			continue
		}
		parsed := Parse(*it.DisplayAt, loc)
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

func hasDisplayAt(items []playlist.PlaylistItem) bool {
	for _, it := range items {
		if it.DisplayAt != nil {
			return true
		}
	}
	return false
}
