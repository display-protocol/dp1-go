package displayat

import (
	"testing"
	"time"
)

func TestParse_DateOnly(t *testing.T) {
	t.Parallel()
	loc, _ := time.LoadLocation("America/New_York")

	tests := []struct {
		raw      string
		wantKind Kind
		wantTime string // RFC3339 in loc
	}{
		{"2026-07-21", KindDateOnly, "2026-07-21T00:00:00"},
		{"2026-01-01", KindDateOnly, "2026-01-01T00:00:00"},
		{"2026-12-31", KindDateOnly, "2026-12-31T00:00:00"},
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
		{KindDateOnly, "date-only"},
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

	p := MustParse("2026-07-21", time.UTC)
	if p.Kind != KindDateOnly {
		t.Errorf("got kind %v, want KindDateOnly", p.Kind)
	}
}
