package displayat

import (
	"testing"
	"time"
)

func TestParse_RejectsDateOnly(t *testing.T) {
	t.Parallel()

	// §3.5.2: date-only YYYY-MM-DD is not an accepted wire form.
	for _, raw := range []string{"2026-07-21", "2026-01-01", "2026-12-31"} {
		p := Parse(raw, time.UTC)
		if p.IsValid() {
			t.Errorf("Parse(%q): expected invalid for date-only, got kind %v", raw, p.Kind)
		}
	}
}

func TestParse_LocalDatetime(t *testing.T) {
	t.Parallel()
	loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")

	tests := []struct {
		raw      string
		wantKind Kind
		wantTime string
	}{
		{"2026-07-21T00:00:00", KindLocal, "2026-07-21T00:00:00"},
		{"2026-07-21T09:30:00", KindLocal, "2026-07-21T09:30:00"},
		{"2026-07-21T23:59:59", KindLocal, "2026-07-21T23:59:59"},
		{"2026-07-21T12:30:45.123", KindLocal, "2026-07-21T12:30:45"},
	}

	for _, tc := range tests {
		p := Parse(tc.raw, loc)
		if p.Kind != tc.wantKind {
			t.Errorf("Parse(%q): got kind %v, want %v", tc.raw, p.Kind, tc.wantKind)
		}
		if !p.IsValid() {
			t.Errorf("Parse(%q): expected valid", tc.raw)
			continue
		}
		got := p.Resolved.In(loc).Format("2006-01-02T15:04:05")
		if got != tc.wantTime {
			t.Errorf("Parse(%q): got time %s, want %s", tc.raw, got, tc.wantTime)
		}
	}
}

func TestParse_Absolute(t *testing.T) {
	t.Parallel()

	tests := []struct {
		raw     string
		wantUTC string
	}{
		{"2026-07-21T00:00:00Z", "2026-07-21T00:00:00Z"},
		{"2026-07-21T09:00:00+07:00", "2026-07-21T02:00:00Z"},
		{"2026-07-21T00:00:00-05:00", "2026-07-21T05:00:00Z"},
		{"2026-07-21T12:30:45.123Z", "2026-07-21T12:30:45Z"},
	}

	for _, tc := range tests {
		p := Parse(tc.raw, time.UTC) // loc ignored for absolute
		if p.Kind != KindAbsolute {
			t.Errorf("Parse(%q): got kind %v, want KindAbsolute", tc.raw, p.Kind)
		}
		if !p.IsValid() {
			t.Errorf("Parse(%q): expected valid", tc.raw)
			continue
		}
		got := p.Resolved.UTC().Format("2006-01-02T15:04:05Z")
		if got != tc.wantUTC {
			t.Errorf("Parse(%q): got UTC %s, want %s", tc.raw, got, tc.wantUTC)
		}
	}
}

func TestParse_Invalid(t *testing.T) {
	t.Parallel()

	invalids := []string{
		"",
		"not-a-date",
		"2026-07-21",               // date-only rejected per §3.5.2
		"2026-13-01",               // invalid month
		"2026-07-32",               // invalid day
		"2026-07-21T25:00:00",      // invalid hour
		"2026-07-21T00:60:00",      // invalid minute
		"2026-07-21T00:00:60",      // invalid second
		"2026-07-21T00:00:00+0700", // compact offset without colon (rejected per spec)
		"2026/07/21",               // wrong separator
	}

	for _, raw := range invalids {
		p := Parse(raw, time.UTC)
		if p.IsValid() {
			t.Errorf("Parse(%q): expected invalid, got %v", raw, p.Kind)
		}
	}
}

func TestParse_AbsoluteIgnoresLoc(t *testing.T) {
	t.Parallel()

	raw := "2026-07-21T09:00:00+07:00"
	locNY, _ := time.LoadLocation("America/New_York")
	locTokyo, _ := time.LoadLocation("Asia/Tokyo")

	p1 := Parse(raw, locNY)
	p2 := Parse(raw, locTokyo)

	if !p1.Resolved.Equal(p2.Resolved) {
		t.Errorf("absolute times should be equal regardless of loc: %v vs %v", p1.Resolved, p2.Resolved)
	}
}

func TestParse_LocalUsesLoc(t *testing.T) {
	t.Parallel()

	raw := "2026-07-21T12:00:00"
	locNY, _ := time.LoadLocation("America/New_York")
	locTokyo, _ := time.LoadLocation("Asia/Tokyo")

	p1 := Parse(raw, locNY)
	p2 := Parse(raw, locTokyo)

	if p1.Resolved.Equal(p2.Resolved) {
		t.Error("local times in different zones should not be equal UTC instants")
	}

	// Both should show same local time in their respective zones.
	if p1.Resolved.In(locNY).Format("15:04:05") != "12:00:00" {
		t.Errorf("NY time wrong: %v", p1.Resolved.In(locNY))
	}
	if p2.Resolved.In(locTokyo).Format("15:04:05") != "12:00:00" {
		t.Errorf("Tokyo time wrong: %v", p2.Resolved.In(locTokyo))
	}
}

func TestKind_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		kind Kind
		want string
	}{
		{KindInvalid, "invalid"},
		{KindLocal, "local"},
		{KindAbsolute, "absolute"},
	}

	for _, tc := range tests {
		if got := tc.kind.String(); got != tc.want {
			t.Errorf("Kind(%d).String() = %q, want %q", tc.kind, got, tc.want)
		}
	}
}

func TestMustParse_Panics(t *testing.T) {
	t.Parallel()

	defer func() {
		if r := recover(); r == nil {
			t.Error("MustParse with invalid input should panic")
		}
	}()

	MustParse("invalid", time.UTC)
}

func TestMustParse_Valid(t *testing.T) {
	t.Parallel()

	p := MustParse("2026-07-21T00:00:00", time.UTC)
	if p.Kind != KindLocal {
		t.Errorf("got kind %v, want KindLocal", p.Kind)
	}
}

func TestParse_NilLocation(t *testing.T) {
	t.Parallel()

	// When loc is nil, should default to UTC.
	p := Parse("2026-07-21T00:00:00", nil)
	if p.Kind != KindLocal {
		t.Errorf("got kind %v, want KindLocal", p.Kind)
	}
	want := "2026-07-21T00:00:00Z"
	got := p.Resolved.UTC().Format(time.RFC3339)
	if got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestParse_AbsoluteWithFractionalAndOffset(t *testing.T) {
	t.Parallel()

	// Fractional seconds with offset (uses fallback layout).
	tests := []struct {
		raw     string
		wantUTC string
	}{
		{"2026-07-21T12:30:45.123456+07:00", "2026-07-21T05:30:45Z"},
		{"2026-07-21T00:00:00.999-05:00", "2026-07-21T05:00:00Z"},
		{"2026-07-21T23:59:59.1+00:00", "2026-07-21T23:59:59Z"},
	}

	for _, tc := range tests {
		p := Parse(tc.raw, nil)
		if p.Kind != KindAbsolute {
			t.Errorf("Parse(%q): got kind %v, want KindAbsolute", tc.raw, p.Kind)
			continue
		}
		got := p.Resolved.UTC().Format("2006-01-02T15:04:05Z")
		if got != tc.wantUTC {
			t.Errorf("Parse(%q): got %s, want %s", tc.raw, got, tc.wantUTC)
		}
	}
}

func TestParse_InvalidCalendarDates(t *testing.T) {
	t.Parallel()

	// These pass regex but fail time.Parse due to invalid calendar values.
	invalids := []string{
		"2026-02-30T00:00:00",       // Feb 30 doesn't exist (local)
		"2026-02-30T00:00:00Z",      // Feb 30 doesn't exist (absolute)
		"2026-04-31T12:00:00+07:00", // Apr 31 doesn't exist
	}

	for _, raw := range invalids {
		p := Parse(raw, time.UTC)
		if p.IsValid() {
			t.Errorf("Parse(%q): expected invalid for bad calendar date, got %v", raw, p.Kind)
		}
	}
}

func TestParse_DSTGapAndFold(t *testing.T) {
	t.Parallel()

	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatalf("LoadLocation: %v", err)
	}

	t.Run("gap_spring_forward", func(t *testing.T) {
		t.Parallel()
		// 2026-03-08: clocks spring forward 02:00 → 03:00. 02:30 does not exist.
		// §3.5.2: resolve to the first valid local instant after the gap (03:00 EDT).
		p := Parse("2026-03-08T02:30:00", loc)
		if !p.IsValid() || p.Kind != KindLocal {
			t.Fatalf("Parse gap: kind=%v valid=%v", p.Kind, p.IsValid())
		}
		got := p.Resolved.In(loc).Format(time.RFC3339)
		want := "2026-03-08T03:00:00-04:00"
		if got != want {
			t.Fatalf("gap resolve: got %s, want %s (unix=%d)", got, want, p.Resolved.Unix())
		}
		// Any wall time inside the gap maps to the same post-gap instant.
		p2 := Parse("2026-03-08T02:00:00", loc)
		if !p2.Resolved.Equal(p.Resolved) {
			t.Fatalf("gap start vs mid: %v vs %v", p2.Resolved, p.Resolved)
		}
	})

	t.Run("fold_fall_back", func(t *testing.T) {
		t.Parallel()
		// 2025-11-02: clocks fall back 02:00 → 01:00. 01:30 occurs twice.
		// §3.5.2: resolve to the earlier ambiguous instant (EDT, -04:00).
		p := Parse("2025-11-02T01:30:00", loc)
		if !p.IsValid() || p.Kind != KindLocal {
			t.Fatalf("Parse fold: kind=%v valid=%v", p.Kind, p.IsValid())
		}
		got := p.Resolved.In(loc).Format(time.RFC3339)
		want := "2025-11-02T01:30:00-04:00"
		if got != want {
			t.Fatalf("fold resolve: got %s, want %s (unix=%d)", got, want, p.Resolved.Unix())
		}
		name, off := p.Resolved.Zone()
		if name != "EDT" || off != -4*3600 {
			t.Fatalf("fold zone: got %s off=%d, want EDT/-14400", name, off)
		}
	})
}

func TestParse_DSTGapMidnight(t *testing.T) {
	t.Parallel()

	// America/Havana springs forward at midnight (00:00 → 01:00) on 2026-03-08.
	// This catches the anchor bug where time.Date(midnight) itself falls in the gap.
	loc, err := time.LoadLocation("America/Havana")
	if err != nil {
		t.Fatalf("LoadLocation: %v", err)
	}

	want := "2026-03-08T01:00:00-04:00"
	for _, raw := range []string{
		"2026-03-08T00:00:00",
		"2026-03-08T00:30:00",
	} {
		p := Parse(raw, loc)
		if !p.IsValid() {
			t.Fatalf("Parse(%q): expected valid gap resolution", raw)
		}
		got := p.Resolved.In(loc).Format(time.RFC3339)
		if got != want {
			t.Fatalf("Parse(%q): got %s, want %s (unix=%d)", raw, got, want, p.Resolved.Unix())
		}
	}
}

func TestParse_FullDayZoneSkip(t *testing.T) {
	t.Parallel()

	// Pacific/Apia skipped 2011-12-30 when moving west of the IDL (§3.5.2 gap →
	// first valid local instant after the gap = 2011-12-31T00:00:00+14:00).
	loc, err := time.LoadLocation("Pacific/Apia")
	if err != nil {
		t.Fatalf("LoadLocation: %v", err)
	}

	want := "2011-12-31T00:00:00+14:00"
	for _, raw := range []string{
		"2011-12-30T00:00:00",
		"2011-12-30T12:00:00",
	} {
		p := Parse(raw, loc)
		if !p.IsValid() {
			t.Fatalf("Parse(%q): expected valid gap resolution", raw)
		}
		got := p.Resolved.In(loc).Format(time.RFC3339)
		if got != want {
			t.Fatalf("Parse(%q): got %s, want %s (unix=%d)", raw, got, want, p.Resolved.Unix())
		}
	}

	// Adjacent real days must still resolve to themselves.
	for _, tc := range []struct {
		raw  string
		want string
	}{
		{"2011-12-29T00:00:00", "2011-12-29T00:00:00-10:00"},
		{"2011-12-31T00:00:00", "2011-12-31T00:00:00+14:00"},
	} {
		p := Parse(tc.raw, loc)
		got := p.Resolved.In(loc).Format(time.RFC3339)
		if got != tc.want {
			t.Fatalf("Parse(%q): got %s, want %s", tc.raw, got, tc.want)
		}
	}
}
