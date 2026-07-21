// Package displayat provides parsing and scheduling helpers for the DP-1 Playlist Extension displayAt field.
//
// Wire formats (ISO 8601 subset per DP-1 Playlist Extension §3.5.2):
//   - Date-only: "2026-07-21" (playback-device local midnight)
//   - Local datetime: "2026-07-21T00:00:00" or with fractional seconds (no timezone)
//   - Absolute: "2026-07-21T00:00:00Z" or with offset like "+07:00"
//
// Parsing resolves date-only and local datetime relative to a provided location (device local);
// absolute forms resolve to exact UTC instants.
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
		t, err := time.ParseInLocation(layout, raw, loc)
		if err != nil {
			return Parsed{Kind: KindInvalid, Raw: raw}
		}
		return Parsed{Kind: KindLocal, Raw: raw, Resolved: t}

	case dateOnlyRE.MatchString(raw):
		t, err := time.ParseInLocation("2006-01-02", raw, loc)
		if err != nil {
			return Parsed{Kind: KindInvalid, Raw: raw}
		}
		return Parsed{Kind: KindDateOnly, Raw: raw, Resolved: t}

	default:
		return Parsed{Kind: KindInvalid, Raw: raw}
	}
}

// MustParse is like Parse but panics on invalid input. Useful for tests.
func MustParse(raw string, loc *time.Location) Parsed {
	p := Parse(raw, loc)
	if !p.IsValid() {
		panic("displayat: invalid value: " + raw)
	}
	return p
}
