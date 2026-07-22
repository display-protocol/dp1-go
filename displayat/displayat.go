// Package displayat provides parsing and scheduling helpers for the DP-1 Playlist Extension displayAt field.
//
// Wire formats (ISO 8601 subset per DP-1 Playlist Extension §3.5.2):
//   - Date-only: "2026-07-21" (playback-device local midnight)
//   - Local datetime: "2026-07-21T00:00:00" or with fractional seconds (no timezone)
//   - Absolute: "2026-07-21T00:00:00Z" or with offset like "+07:00"
//
// Parsing resolves date-only and local datetime relative to a provided location (device local);
// absolute forms resolve to exact UTC instants.
//
// DST gap/fold for timezone-less values follow §3.5.2: gap → first valid local instant after
// the gap; fold → earlier of the two ambiguous instants.
package displayat

import (
	"regexp"
	"time"
)

// Kind distinguishes how a displayAt value should be interpreted.
type Kind int

const (
	// KindInvalid means the value could not be parsed.
	KindInvalid Kind = iota
	// KindDateOnly means "YYYY-MM-DD" interpreted as local midnight.
	KindDateOnly
	// KindLocal means datetime without timezone, interpreted in device local time.
	KindLocal
	// KindAbsolute means datetime with Z or offset, a fixed UTC instant.
	KindAbsolute
)

func (k Kind) String() string {
	switch k {
	case KindDateOnly:
		return "date-only"
	case KindLocal:
		return "local"
	case KindAbsolute:
		return "absolute"
	default:
		return "invalid"
	}
}

// Parsed holds the result of parsing a displayAt wire value.
type Parsed struct {
	Kind Kind
	Raw  string

	// Resolved is the parsed time. For KindDateOnly and KindLocal, this is resolved
	// in the location passed to Parse. For KindAbsolute, it's the exact UTC instant.
	// Zero if Kind == KindInvalid.
	Resolved time.Time
}

// IsValid reports whether parsing succeeded.
func (p Parsed) IsValid() bool {
	return p.Kind != KindInvalid
}

var (
	// Patterns aligned with schema.json DisplayAt oneOf branches.
	dateOnlyRE = regexp.MustCompile(`^\d{4}-(0[1-9]|1[0-2])-(0[1-9]|[12]\d|3[01])$`)
	localRE    = regexp.MustCompile(`^\d{4}-(0[1-9]|1[0-2])-(0[1-9]|[12]\d|3[01])T([01]\d|2[0-3]):[0-5]\d:[0-5]\d(\.\d+)?$`)
	absoluteRE = regexp.MustCompile(`^\d{4}-(0[1-9]|1[0-2])-(0[1-9]|[12]\d|3[01])T([01]\d|2[0-3]):[0-5]\d:[0-5]\d(\.\d+)?(Z|[+-]([01]\d|2[0-3]):[0-5]\d)$`)
)

// Parse parses a displayAt wire value and resolves it to a time.Time.
// For date-only and local datetime forms, loc determines interpretation (pass device local timezone).
// For absolute forms (Z or offset), loc is ignored.
//
// Returns Parsed with Kind == KindInvalid if the value doesn't match any accepted pattern
// or if the calendar/clock values are invalid (e.g., Feb 30).
func Parse(raw string, loc *time.Location) Parsed {
	if loc == nil {
		loc = time.UTC
	}

	switch {
	case absoluteRE.MatchString(raw):
		t, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			t, err = time.Parse("2006-01-02T15:04:05.999999999Z07:00", raw)
		}
		if err != nil {
			return Parsed{Kind: KindInvalid, Raw: raw}
		}
		return Parsed{Kind: KindAbsolute, Raw: raw, Resolved: t.UTC()}

	case localRE.MatchString(raw):
		layout := "2006-01-02T15:04:05"
		if len(raw) > 19 {
			layout = "2006-01-02T15:04:05.999999999"
		}
		// Parse as UTC to extract calendar/clock components without applying loc yet.
		// time.Parse rejects invalid calendar dates; DST gap/fold are handled in resolveLocalWall.
		wall, err := time.Parse(layout, raw)
		if err != nil {
			return Parsed{Kind: KindInvalid, Raw: raw}
		}
		y, m, d := wall.Date()
		hh, mm, ss := wall.Clock()
		resolved := resolveLocalWall(y, m, d, hh, mm, ss, wall.Nanosecond(), loc)
		return Parsed{Kind: KindLocal, Raw: raw, Resolved: resolved}

	case dateOnlyRE.MatchString(raw):
		wall, err := time.Parse("2006-01-02", raw)
		if err != nil {
			return Parsed{Kind: KindInvalid, Raw: raw}
		}
		y, m, d := wall.Date()
		resolved := resolveLocalWall(y, m, d, 0, 0, 0, 0, loc)
		return Parsed{Kind: KindDateOnly, Raw: raw, Resolved: resolved}

	default:
		return Parsed{Kind: KindInvalid, Raw: raw}
	}
}

// resolveLocalWall resolves a timezone-less wall time in loc per DP-1 §3.5.2:
//   - Gap (spring-forward; local time does not exist): first valid local instant after the gap.
//   - Fold (fall-back; local time occurs twice): earlier of the two ambiguous instants.
//
// time.Date / ParseInLocation alone are not sufficient: on gap times, Go currently maps into
// the pre-transition offset (wrong for §3.5.2).
func resolveLocalWall(year int, month time.Month, day, hour, min, sec, nsec int, loc *time.Location) time.Time {
	candidates := wallUTCCandidates(year, month, day, hour, min, sec, nsec, loc)
	switch len(candidates) {
	case 0:
		return firstInstantAfterGap(year, month, day, loc)
	case 1:
		return candidates[0]
	default:
		earliest := candidates[0]
		for _, c := range candidates[1:] {
			if c.Before(earliest) {
				earliest = c
			}
		}
		return earliest
	}
}

// wallUTCCandidates returns distinct instants in loc that display as the given wall clock.
// Zero candidates means the wall time falls in a DST gap; two means a fold.
func wallUTCCandidates(year int, month time.Month, day, hour, min, sec, nsec int, loc *time.Location) []time.Time {
	offsets := nearbyOffsets(year, month, day, loc)
	seen := make(map[int64]struct{}, len(offsets))
	out := make([]time.Time, 0, len(offsets))
	for _, off := range offsets {
		fz := time.FixedZone("", off)
		nominal := time.Date(year, month, day, hour, min, sec, nsec, fz)
		got := nominal.UTC().In(loc)
		if !sameWall(got, year, month, day, hour, min, sec) || got.Nanosecond() != nsec {
			continue
		}
		key := got.UnixNano()
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, got)
	}
	return out
}

// nearbyOffsets collects timezone offsets around the given civil day (day±1 noon and midnight).
func nearbyOffsets(year int, month time.Month, day int, loc *time.Location) []int {
	points := []time.Time{
		time.Date(year, month, day, 0, 0, 0, 0, loc),
		time.Date(year, month, day, 12, 0, 0, 0, loc),
		time.Date(year, month, day, 23, 0, 0, 0, loc),
		time.Date(year, month, day, 12, 0, 0, 0, loc).Add(-24 * time.Hour),
		time.Date(year, month, day, 12, 0, 0, 0, loc).Add(24 * time.Hour),
	}
	seen := make(map[int]struct{}, len(points))
	out := make([]int, 0, len(points))
	for _, p := range points {
		_, off := p.Zone()
		if _, ok := seen[off]; ok {
			continue
		}
		seen[off] = struct{}{}
		out = append(out, off)
	}
	return out
}

// firstInstantAfterGap returns the first valid local instant after the spring-forward gap
// on the given civil day in loc. Used when the requested wall time does not exist.
//
// Do not anchor at local midnight of the target day: in zones where the gap includes
// midnight (e.g. America/Havana 00:00→01:00), time.Date(..., 0,0,0, loc) itself lands
// in the gap and Go maps it to the previous evening, which breaks a forward-only SOD scan.
func firstInstantAfterGap(year int, month time.Month, day int, loc *time.Location) time.Time {
	// Noon on the previous civil day is a stable pre-transition anchor.
	anchor := time.Date(year, month, day, 12, 0, 0, 0, loc).Add(-24 * time.Hour)
	prev := anchor
	for i := 1; i <= 36*3600; i++ {
		cur := anchor.Add(time.Duration(i) * time.Second)
		prevSOD := secondsOfDay(prev.In(loc))
		curSOD := secondsOfDay(cur.In(loc))
		normalStep := curSOD == prevSOD+1
		// Civil midnight without a DST gap: 23:59:59 → 00:00:00.
		normalMidnight := prevSOD == 23*3600+59*60+59 && curSOD == 0
		if !normalStep && !normalMidnight {
			// Spring-forward discontinuity (e.g. 01:59:59→03:00:00 or 23:59:59→01:00:00).
			return cur.In(loc)
		}
		prev = cur
	}
	// Fallback: should be unreachable for real IANA zones with a spring-forward gap.
	return time.Date(year, month, day, 12, 0, 0, 0, loc)
}

func sameWall(t time.Time, year int, month time.Month, day, hour, min, sec int) bool {
	y, m, d := t.Date()
	h, mi, s := t.Clock()
	return y == year && m == month && d == day && h == hour && mi == min && s == sec
}

func secondsOfDay(t time.Time) int {
	h, m, s := t.Clock()
	return h*3600 + m*60 + s
}

// MustParse is like Parse but panics on invalid input. Useful for tests.
func MustParse(raw string, loc *time.Location) Parsed {
	p := Parse(raw, loc)
	if !p.IsValid() {
		panic("displayat: invalid value: " + raw)
	}
	return p
}
