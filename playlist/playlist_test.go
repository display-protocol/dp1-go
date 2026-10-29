package playlist

import (
	"bytes"
	"encoding/json"
	"runtime"
	"strings"
	"testing"

	"github.com/display-protocol/dp1-go/extension/contentrating"
	"github.com/display-protocol/dp1-go/extension/identity"
	"github.com/display-protocol/dp1-go/extension/playlists"
)

func TestPlaylist_JSONRoundTrip(t *testing.T) {
	t.Parallel()
	tru := true
	pre := 5.0
	p := Playlist{
		DPVersion: "1.1.0",
		ID:        "385f79b6-a45f-4c1c-8080-e93a192adccc",
		Title:     "T",
		Note:      &playlists.Note{Text: "Show intro", Duration: &pre},
		Defaults: &Defaults{
			Display: &DisplayPrefs{Scaling: "fit", Autoplay: &tru},
			License: "open",
		},
		Items: []PlaylistItem{
			{
				Source:  "https://a",
				License: "token",
				Display: &DisplayPrefs{Scaling: "fill", ImageRendering: "pixelated"},
				Note:    &playlists.Note{Text: "Item card"},
				Provenance: &ProvenanceBlock{
					Type: ProvenanceOnChain,
					Contract: &ProvenanceContract{
						Chain: "evm", Standard: "erc721", Address: "0xabc",
					},
				},
			},
		},
		Signatures: []Signature{{
			Alg: "ed25519", Kid: "did:key:z", Ts: "2025-01-01T00:00:00Z",
			PayloadHash: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			Role:        RoleCurator, Sig: "abc",
		}},
	}
	b, err := json.Marshal(&p)
	if err != nil {
		t.Fatal(err)
	}
	var out Playlist
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	if out.Title != p.Title || len(out.Items) != 1 {
		t.Fatalf("%+v", out)
	}
	if out.Note == nil || out.Note.Text != "Show intro" || out.Note.Duration == nil || *out.Note.Duration != 5 {
		t.Fatalf("playlist note: %+v", out.Note)
	}
	if out.Items[0].Note == nil || out.Items[0].Note.Text != "Item card" {
		t.Fatalf("item note: %+v", out.Items[0].Note)
	}
	if out.Items[0].Display == nil || out.Items[0].Display.ImageRendering != "pixelated" {
		t.Fatalf("item display imageRendering: %+v", out.Items[0].Display)
	}
}

// TestSeriesID_AcceptsIntOrDecimalString locks in the schema's oneOf [integer, string]
// for provenance.contract.seriesId: small ids are commonly sent as a JSON number, large
// ones (beyond float64's exact range) as a decimal string — both must decode, and the
// type always re-marshals as a string (schema-valid either way, and never risks the
// precision loss a round-tripped large integer would).
func TestSeriesID_AcceptsIntOrDecimalString(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		json string
		want SeriesID
	}{
		{"small integer", `5`, "5"},
		{"zero", `0`, "0"},
		{"decimal string", `"115792089237316195423570985008687907853269984665640564039457584007913129639935"`, "115792089237316195423570985008687907853269984665640564039457584007913129639935"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var got SeriesID
			if err := json.Unmarshal([]byte(tc.json), &got); err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
			b, err := json.Marshal(got)
			if err != nil {
				t.Fatal(err)
			}
			if string(b) != `"`+string(tc.want)+`"` {
				t.Fatalf("marshal: got %s", b)
			}
		})
	}
}

// TestSeriesID_RejectsNonDigits is the negative half of the test above. The schema's
// oneOf is `{integer, minimum 0}` or `{string, ^[0-9]+$}`, and UnmarshalJSON is reachable
// without the schema — a caller that json.Unmarshals a Playlist directly, which README
// documents as supported. If the two disagree, this SDK decodes a value it would itself
// refuse to validate and re-emits it as a string that fails the oneOf: a document it
// produced and rejects. Each case below decoded cleanly before the digit guard.
func TestSeriesID_RejectsNonDigits(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		json string
	}{
		{"negative string", `"-5"`},
		{"negative number", `-5`},
		{"fractional string", `"5.5"`},
		{"fractional number", `5.5`},
		{"empty string", `""`},
		{"non-numeric string", `"abc"`},
		{"hex string", `"0x10"`},
		{"leading plus", `"+5"`},
		{"whitespace padded", `" 5 "`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var got SeriesID
			if err := json.Unmarshal([]byte(tc.json), &got); err == nil {
				t.Fatalf("decoded %s as %q, want an error", tc.json, got)
			}
		})
	}
}

// TestSeriesID_RejectsExponentBlowup locks the digit bound that keeps SeriesID's big.Rat
// path from turning a tiny document into seconds of CPU and a gigabyte of allocation.
//
// The assertion is an error, not a deadline, and that is deliberate: a timing assertion
// would be flaky, while an error is proof on its own. Every literal below IS a
// non-negative integer, so if big.Rat were still reached it would expand the value,
// IsInt would hold, the digits would pass seriesIDDigits, and Unmarshal would return
// nil. An error can therefore only come from the bound refusing the literal first.
//
// Note what does not work: bounding the literal's own length. "1e1000000" is nine bytes.
func TestSeriesID_RejectsExponentBlowup(t *testing.T) {
	t.Parallel()
	cases := []string{
		`1e1000000`,
		`1e2000`,
		`1E+5000`,
		`1e99999999999999999999`, // exponent itself overflows an int
	}
	for _, in := range cases {
		t.Run(in[:min(len(in), 24)], func(t *testing.T) {
			t.Parallel()
			var got SeriesID
			if err := json.Unmarshal([]byte(in), &got); err == nil {
				t.Fatalf("decoded a %d-byte literal as %d digits, want an error", len(in), len(got))
			}
		})
	}
}

// TestSeriesID_RejectsNegativeExponentBlowup is the other half of the test above, for the
// amplification that reaches big.Rat through the denominator instead of the numerator.
//
// Why this one cannot assert only an error, as the positive case does: `1e-1000000` is a
// fraction, so IsInt refuses it whether or not the bound ran — a returned error proves
// nothing about which of the two refused it, and the guard could regress to `exponent = 0`
// with every error assertion still green. What the finding is about is the COST of the
// refusal, so the cost is what is asserted. big.Rat.SetString materializes a 10^k
// denominator before IsInt is ever consulted: ~2.9 MB for the first literal below, from
// ten bytes on the wire.
//
// TotalAlloc is cumulative and monotonic, so an intervening GC cannot perturb the delta,
// and the 64 KiB ceiling sits ~45x under the unguarded cost and ~30x over the guarded one
// — wide enough that allocator noise cannot reach it from either side.
//
// It is also PROCESS-WIDE, which is why neither this test nor its subtests may call
// t.Parallel(): any other test allocating between the two ReadMemStats calls lands in the
// delta and is charged to this literal. Under `go test ./playlist -race -count=10` that
// read up to ~390 KB against a 64 KiB ceiling and failed on correct code, reporting a
// big.Rat regression that had not happened. Go releases parked parallel tests only after
// the sequential pass finishes, so a test that never parks is the isolation — no
// -parallel=1 at the call site required. Keep the measurement sequential if cases are
// added; the same applies to the sibling measurement in parse_test.go.
func TestSeriesID_RejectsNegativeExponentBlowup(t *testing.T) {
	const maxAlloc = 64 << 10
	cases := []string{
		`1e-1000000`,
		`1e-100000`,
		`1E-5000`,
		`1.5e-900000`,
	}
	for _, in := range cases {
		t.Run(in, func(t *testing.T) {
			if seriesIDWithinDigitBound(in) {
				t.Fatalf("bound admitted %q (%d bytes); big.Rat would build a denominator of 10^|exponent| for it", in, len(in))
			}
			var before, after runtime.MemStats
			runtime.ReadMemStats(&before)
			var got SeriesID
			err := json.Unmarshal([]byte(in), &got)
			runtime.ReadMemStats(&after)
			if err == nil {
				t.Fatalf("decoded %q as %q, want an error", in, got)
			}
			if used := after.TotalAlloc - before.TotalAlloc; used > maxAlloc {
				t.Fatalf("rejecting %q (%d bytes) allocated %d bytes, want under %d: the bound let it reach big.Rat",
					in, len(in), used, maxAlloc)
			}
		})
	}
}

// The negative branch bounds the mantissa's own length as well as the exponent's
// magnitude, and that half of it is what keeps the branch from accepting more than the
// base branch did. A mantissa over the budget is refused whatever its exponent absorbs:
// "1" followed by 1024 zeros times 10^-512 is an ordinary 513-digit integer, and it was
// refused on v0.7.0 too (the mantissa alone was already over 1024), so it stays refused.
//
// The accepted case sits one character under the limit, so the two together pin the
// boundary rather than just one side of it. Without this test the length check is the one
// line of the guard no case reaches, and a fix that dropped it would look green.
func TestSeriesID_BoundsMantissaLengthUnderNegativeExponent(t *testing.T) {
	t.Parallel()
	atLimit := "1" + strings.Repeat("0", 1023) + "e-512"   // 1024 mantissa characters
	overLimit := "1" + strings.Repeat("0", 1024) + "e-512" // 1025 — one too many

	if !seriesIDWithinDigitBound(atLimit) {
		t.Fatalf("bound refused a %d-character mantissa; the limit is %d", len(atLimit)-5, maxSeriesIDDigits)
	}
	var got SeriesID
	if err := json.Unmarshal([]byte(atLimit), &got); err != nil {
		t.Fatalf("a 1024-character mantissa under 10^-512 is an integer and must decode: %v", err)
	}
	if want := "1" + strings.Repeat("0", 511); string(got) != want {
		t.Fatalf("decoded to %d digits, want %d", len(got), len(want))
	}

	if seriesIDWithinDigitBound(overLimit) {
		t.Fatalf("bound admitted a %d-character mantissa; the limit is %d", len(overLimit)-5, maxSeriesIDDigits)
	}
	if err := json.Unmarshal([]byte(overLimit), &got); err == nil {
		t.Fatalf("decoded a %d-character mantissa, want an error", len(overLimit)-5)
	}
}

// A zero mantissa is the integer 0 however large its exponent, on either sign, and the
// bound accepts it without consulting the magnitude at all. That is safe only because
// big.Rat declines to do the work: SetString returns at its own zero check before any
// exponent is applied, so `0e-1000000` costs ~48 bytes, the same as `0e-5`. This test
// locks that short-circuit rather than the bound's return value, because the bound is
// deliberately permissive here and the short-circuit is the whole reason it may be — if a
// future math/big built the 10^k denominator before looking at the mantissa, a zero
// mantissa would become the same amplification vector this guard exists to refuse, and
// nothing else in the suite would notice.
//
// Both signs are covered because the two reached this state differently, and the positive
// side is the one that regressed. `0e-5000` plainly decoded to "0" on v0.7.0. So did
// `0e9223372036854775807` — but only because len(mantissa)+exponent overflowed int and
// wrapped into range, the very hole the range check closed. Closing it without exempting
// the zero mantissa turned a literal that decoded into a hard refusal, which is why the
// exemption is not gated on the exponent's magnitude or the mantissa's length.
//
// Sequential for the same reason as the measurement above: TotalAlloc is process-wide.
func TestSeriesID_ZeroMantissaCostsNothing(t *testing.T) {
	const maxAlloc = 64 << 10
	for _, in := range []string{
		`0e-5000`, `0e-1000000`, `-0e-5000`, `0.0e-1000000`, `0E-2000`,
		`0e5000`, `0E+2000`, `0.0e1000000`,
		// The overflow band: on v0.7.0 these passed the bound only because the sum
		// wrapped negative, and big.Rat read them as 0 regardless. An exponent one past
		// math.MaxInt (`0e9223372036854775808`) is NOT here: strconv.Atoi refuses it
		// before any of this, exactly as it does on the base branch, so nothing regressed
		// there and the zero mantissa never gets a say.
		`0e9223372036854775807`, `-0e9223372036854775807`, `0e-9223372036854775808`,
	} {
		t.Run(in, func(t *testing.T) {
			if !seriesIDWithinDigitBound(in) {
				t.Fatalf("bound refused %q; it is the integer 0 and big.Rat applies no exponent to it", in)
			}
			var before, after runtime.MemStats
			runtime.ReadMemStats(&before)
			var got SeriesID
			err := json.Unmarshal([]byte(in), &got)
			runtime.ReadMemStats(&after)
			if err != nil {
				t.Fatalf("%s is the integer 0 and decoded on v0.7.0; it must still decode: %v", in, err)
			}
			if got != "0" {
				t.Fatalf("%s decoded to %q, want \"0\"", in, got)
			}
			if used := after.TotalAlloc - before.TotalAlloc; used > maxAlloc {
				t.Fatalf("decoding %q allocated %d bytes, want under %d: big.Rat applied the exponent to a zero mantissa",
					in, used, maxAlloc)
			}
		})
	}
}

// An exponent at either end of int must be refused by the bound itself, not by whatever
// runs after it. Go's signed overflow wraps rather than traps, so arithmetic on the
// parsed exponent before it is range-checked silently admits the literal:
// -math.MinInt64 is math.MinInt64 again, and math.MaxInt64 + len(mantissa) wraps
// negative — both then satisfy `<= maxSeriesIDDigits`.
//
// This asserts the bound's own return value and deliberately not an allocation, because
// there is no allocation to measure: big.Rat.SetString refuses an exponent this large on
// its own, for about 1.5 KB, so a bypass here costs nothing today. That is exactly why it
// needs a test of its own — the guard is currently correct only by accident of big.Rat's
// internal limit, and nothing would notice if that limit moved. Severity is low on its
// own; the test is cheap and the guard should be total.
func TestSeriesID_BoundRefusesExponentsAtIntLimits(t *testing.T) {
	t.Parallel()
	for _, in := range []string{`1e9223372036854775807`, `1e-9223372036854775808`} {
		t.Run(in, func(t *testing.T) {
			t.Parallel()
			if seriesIDWithinDigitBound(in) {
				t.Fatalf("bound admitted %q (%d bytes); the exponent overflowed int before the comparison", in, len(in))
			}
			var got SeriesID
			if err := json.Unmarshal([]byte(in), &got); err == nil {
				t.Fatalf("decoded %q as %q, want an error", in, got)
			}
		})
	}
}

// A negative exponent the mantissa really does absorb is an ordinary integer and must
// keep decoding: bounding on the exponent's magnitude must not turn 1500e-2 into an
// error just because the sign is now counted.
//
// The long case is the one that matters, and the one the first cut of this fix got wrong:
// the short literals pass under any formulation, because 4 + 2 is inside the bound
// whether the exponent is summed with the mantissa or not. "1" followed by 512 zeros,
// times 10^-512, is the integer 1 — 513 mantissa characters against a 512 exponent, which
// summed is 1025 and over the 1024 budget. It decoded on v0.7.0 and must keep decoding;
// a formulation that sums the two refuses it and only the long case notices.
func TestSeriesID_AcceptsAbsorbedNegativeExponent(t *testing.T) {
	t.Parallel()
	cases := map[string]SeriesID{`1500e-2`: "15", `100e-2`: "1", `1E-0`: "1"}
	cases["1"+strings.Repeat("0", 512)+"e-512"] = "1"
	for in, want := range cases {
		t.Run(in, func(t *testing.T) {
			t.Parallel()
			var got SeriesID
			if err := json.Unmarshal([]byte(in), &got); err != nil {
				t.Fatalf("%s must still decode: %v", in, err)
			}
			if got != want {
				t.Fatalf("%s decoded to %q, want %q", in, got, want)
			}
		})
	}
}

// The bound must not cost a real id its place: 2^256-1 is the largest value the string
// form exists for, and the equivalent exponent spellings stay well inside it.
func TestSeriesID_AcceptsLargeRealisticIDs(t *testing.T) {
	t.Parallel()
	const max256 = "115792089237316195423570985008687907853269984665640564039457584007913129639935"
	var got SeriesID
	if err := json.Unmarshal([]byte(`"`+max256+`"`), &got); err != nil {
		t.Fatal(err)
	}
	if string(got) != max256 {
		t.Fatalf("got %q", got)
	}
	// 1e1000 expands to 1001 digits, just inside maxSeriesIDDigits — the boundary is
	// above anything a chain mints, not just above the test fixtures.
	if err := json.Unmarshal([]byte(`1e1000`), &got); err != nil {
		t.Fatalf("1e1000 must stay inside the bound: %v", err)
	}
	if len(got) != 1001 {
		t.Fatalf("1e1000 expanded to %d digits", len(got))
	}
	// A long literal written out in full is deliberately NOT bounded, unlike the
	// exponent form: it costs only as much work as it costs bytes, so there is nothing
	// to amplify, and refusing it would make this decoder reject documents the schema
	// accepts — neither branch of the schema's oneOf carries a maxLength. The string
	// form below is unbounded for the same reason and must stay that way.
	long := "9" + strings.Repeat("0", 2000)
	if err := json.Unmarshal([]byte(long), &got); err != nil {
		t.Fatalf("a written-out literal must not hit the exponent bound: %v", err)
	}
	if err := json.Unmarshal([]byte(`"`+long+`"`), &got); err != nil {
		t.Fatalf("the string form must stay unbounded: %v", err)
	}
}

// Exponent spellings are a JSON number the schema's integer branch accepts, so they must
// survive the digit guard added for the negatives above — the guard runs on the
// canonicalized digits, not on the producer's raw spelling.
func TestSeriesID_CanonicalizesExponentForms(t *testing.T) {
	t.Parallel()
	cases := map[string]SeriesID{`5e2`: "500", `1E+3`: "1000", `5.0`: "5"}
	for in, want := range cases {
		t.Run(in, func(t *testing.T) {
			t.Parallel()
			var got SeriesID
			if err := json.Unmarshal([]byte(in), &got); err != nil {
				t.Fatal(err)
			}
			if got != want {
				t.Fatalf("got %q want %q", got, want)
			}
		})
	}
}

// TestPlaylist_OpenAtAndLinks locks in dp1#55: OpenAt is a playlist-level fact distinct
// from item-level DisplayAt scheduling (TestPlaylist_DisplayAt below exercises that one),
// and Links/curators carry the fuller Entity shape (role/location/addresses/avatar/
// biographies/links), not just name+key+url.
func TestPlaylist_OpenAtAndLinks(t *testing.T) {
	t.Parallel()
	p := Playlist{
		DPVersion: "1.1.0",
		Title:     "T",
		Items:     []PlaylistItem{{Source: "https://a", License: "token"}},
		OpenAt:    "2026-07-21T18:00:00-04:00",
		Curators: []identity.Entity{{
			Name:        "A Curator",
			Key:         "did:key:z6Mk",
			Role:        "curator",
			Location:    "New York, USA",
			Addresses:   []string{"0xabc"},
			Avatar:      &identity.Thumbnail{URI: "https://a/avatar.png"},
			Biographies: []identity.Biography{{Text: "Bio text"}},
			Links:       []identity.Link{{Type: identity.LinkTypeWebsite, URL: "https://a"}},
		}},
		Links: []identity.Link{{
			Type:    identity.LinkTypeArticle,
			URL:     "https://press.example/review",
			Title:   "A review",
			Date:    "2026-07-20T00:00:00Z",
			Authors: []identity.Entity{{Name: "A Writer"}},
		}},
	}
	b, err := json.Marshal(&p)
	if err != nil {
		t.Fatal(err)
	}
	var out Playlist
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	if out.OpenAt != p.OpenAt {
		t.Fatalf("openAt: got %q want %q", out.OpenAt, p.OpenAt)
	}
	if len(out.Curators) != 1 || out.Curators[0].Role != "curator" || out.Curators[0].Location != "New York, USA" {
		t.Fatalf("curator: %+v", out.Curators)
	}
	if len(out.Curators[0].Biographies) != 1 || out.Curators[0].Biographies[0].Text != "Bio text" {
		t.Fatalf("curator biographies: %+v", out.Curators[0].Biographies)
	}
	if len(out.Links) != 1 || out.Links[0].Title != "A review" || len(out.Links[0].Authors) != 1 {
		t.Fatalf("playlist links: %+v", out.Links)
	}
}

func strPtr(s string) *string { return &s }

func ratingPtr(r contentrating.Rating) *contentrating.Rating { return &r }

func displayAtString(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func TestPlaylist_DisplayAt(t *testing.T) {
	t.Parallel()

	p := Playlist{
		DPVersion: "1.1.0",
		ID:        "385f79b6-a45f-4c1c-8080-e93a192adccc",
		Title:     "Daily",
		Items: []PlaylistItem{
			{
				ID:     "1",
				Title:  "Intro",
				Source: "https://cdn.example.com/intro.html",
			},
			{
				ID:        "2",
				Title:     "Day 1",
				Source:    "https://cdn.example.com/day1.html",
				DisplayAt: strPtr("2026-07-21T00:00:00"),
			},
			{
				ID:        "3",
				Title:     "Day 2",
				Source:    "https://cdn.example.com/day2.html",
				DisplayAt: strPtr("2026-07-22T00:00:00"),
			},
			{
				ID:        "4",
				Title:     "Day 3",
				Source:    "https://cdn.example.com/day3.html",
				DisplayAt: strPtr("2026-07-23T00:00:00Z"),
			},
		},
		Signatures: []Signature{{
			Alg: "ed25519", Kid: "did:key:z", Ts: "2025-01-01T00:00:00Z",
			PayloadHash: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			Role:        RoleCurator, Sig: "abc",
		}},
	}

	b, err := json.Marshal(&p)
	if err != nil {
		t.Fatal(err)
	}

	var out Playlist
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}

	if len(out.Items) != 4 {
		t.Fatalf("expected 4 items, got %d", len(out.Items))
	}

	expected := []string{"", "2026-07-21T00:00:00", "2026-07-22T00:00:00", "2026-07-23T00:00:00Z"}
	for i, want := range expected {
		if displayAtString(out.Items[i].DisplayAt) != want {
			t.Errorf("item[%d].DisplayAt = %q, want %q", i, displayAtString(out.Items[i].DisplayAt), want)
		}
		if want == "" && out.Items[i].DisplayAt != nil {
			t.Errorf("item[%d].DisplayAt want nil (absent), got non-nil", i)
		}
	}
}

func TestPlaylistItem_DisplayAt_Formats(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		displayAt string
	}{
		{"local_datetime", "2026-07-21T00:00:00"},
		{"local_with_frac", "2026-07-21T00:00:00.123"},
		{"absolute_Z", "2026-07-21T00:00:00Z"},
		{"absolute_positive_offset", "2026-07-21T09:00:00+07:00"},
		{"absolute_negative_offset", "2026-07-21T00:00:00-05:00"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			item := PlaylistItem{
				Source:    "https://example.com/a",
				DisplayAt: strPtr(tc.displayAt),
			}
			b, err := json.Marshal(&item)
			if err != nil {
				t.Fatal(err)
			}
			var out PlaylistItem
			if err := json.Unmarshal(b, &out); err != nil {
				t.Fatal(err)
			}
			if displayAtString(out.DisplayAt) != tc.displayAt {
				t.Errorf("got %q, want %q", displayAtString(out.DisplayAt), tc.displayAt)
			}
		})
	}
}

func TestPlaylistItem_DisplayAt_NullAndEmpty(t *testing.T) {
	t.Parallel()

	var nullItem PlaylistItem
	if err := json.Unmarshal([]byte(`{"source":"https://a","displayAt":null}`), &nullItem); err != nil {
		t.Fatal(err)
	}
	if nullItem.DisplayAt != nil {
		t.Fatalf("null displayAt: got %q, want nil", *nullItem.DisplayAt)
	}

	var emptyItem PlaylistItem
	if err := json.Unmarshal([]byte(`{"source":"https://a","displayAt":""}`), &emptyItem); err != nil {
		t.Fatal(err)
	}
	if emptyItem.DisplayAt == nil || *emptyItem.DisplayAt != "" {
		t.Fatalf("empty displayAt: got %#v, want pointer to empty string", emptyItem.DisplayAt)
	}
}

// InlineManifest is the playlists-extension carriage of a full ref manifest (§3.6). Because it
// is raw JSON the wire form survives a round trip byte for byte — including the artist "id": ""
// of the §3.6 example, which a decoded manifest would drop through omitempty and thereby change
// the JCS payload of a signed playlist.
func TestPlaylistItem_InlineManifest_RoundTripIsByteFaithful(t *testing.T) {
	t.Parallel()
	manifest := `{"refVersion":"0.1.0","id":"ref-9d26ecb3","created":"2026-07-28T00:00:00Z","locale":"en",` +
		`"metadata":{"title":"Pre-Process","artists":[{"name":"Casey Reas","id":""}],` +
		`"thumbnails":{"default":{"uri":"https://example.com/thumb.png","w":1200,"h":900},` +
		`"small":{"uri":"https://example.com/thumb-s.png"}}}}`
	wire := []byte(`{"source":"https://example.com/work.html","inlineManifest":` + manifest + `}`)

	var item PlaylistItem
	if err := json.Unmarshal(wire, &item); err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(&item)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(b, wire) {
		t.Fatalf("round trip changed the bytes:\n want %s\n got  %s", wire, b)
	}

	m, err := item.ParseInlineManifest()
	if err != nil {
		t.Fatal(err)
	}
	if m == nil || m.ID != "ref-9d26ecb3" {
		t.Fatalf("inlineManifest: %+v", m)
	}
	th := m.Metadata.Thumbnails
	if th["default"].W == nil || *th["default"].W != 1200 {
		t.Fatalf("default thumbnail width: %+v", th["default"])
	}
	if th["small"].W != nil || th["small"].H != nil {
		t.Fatalf("expected absent dimensions to stay absent: %+v", th["small"])
	}

	bare, err := json.Marshal(&PlaylistItem{Source: "https://example.com/work.html"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(bare), "inlineManifest") {
		t.Fatalf("unset inlineManifest must be omitted: %s", bare)
	}
}

// A core-parsed playlist can carry any JSON here, so decoding must fail loudly rather than
// silently yielding a nil manifest that reads as "no manifest present".
func TestPlaylistItem_ParseInlineManifest_absentAndInvalid(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{"", "null"} {
		item := PlaylistItem{Source: "https://a", InlineManifest: json.RawMessage(raw)}
		m, err := item.ParseInlineManifest()
		if err != nil || m != nil {
			t.Fatalf("%q must read as absent, got %+v err=%v", raw, m, err)
		}
	}
	for _, raw := range []string{`"https://m.example/x.json"`, `[]`, `{"metadata":{"thumbnails":{"default":{"w":"1200"}}}}`} {
		item := PlaylistItem{Source: "https://a", InlineManifest: json.RawMessage(raw)}
		if _, err := item.ParseInlineManifest(); err == nil {
			t.Fatalf("%s must fail to decode", raw)
		}
	}
}

// PlaylistItem has a custom UnmarshalJSON, so it must keep encoding/json's merge semantics:
// decoding writes only the members the document carries and leaves the rest of the receiver
// alone, and a literal null is a no-op. Callers depend on this when they decode into a reused
// variable, and json itself depends on it when it reuses the existing backing array of a
// non-empty Items slice. A shadow struct seeded from zero instead of from the receiver would
// silently blank Title, DisplayAt, InlineManifest and the rating on a partial document.
func TestPlaylistItemUnmarshalMergesIntoReceiver(t *testing.T) {
	t.Parallel()

	t.Run("partial document keeps omitted fields", func(t *testing.T) {
		t.Parallel()
		reasons := []string{"nudity"}
		it := PlaylistItem{
			Source:         "https://old",
			Title:          "Old Title",
			DisplayAt:      strPtr("2026-01-01T00:00:00Z"),
			InlineManifest: json.RawMessage(`{"refVersion":"0.1.0"}`),
			ContentRating:  ratingPtr(contentrating.RatingMature),
			ContentReasons: &reasons,
		}
		if err := json.Unmarshal([]byte(`{"source":"https://new"}`), &it); err != nil {
			t.Fatal(err)
		}
		if it.Source != "https://new" {
			t.Fatalf("source: %q", it.Source)
		}
		if it.Title != "Old Title" {
			t.Fatalf("title must survive an omitted member, got %q", it.Title)
		}
		if it.DisplayAt == nil || *it.DisplayAt != "2026-01-01T00:00:00Z" {
			t.Fatalf("displayAt: %v", it.DisplayAt)
		}
		if len(it.InlineManifest) == 0 {
			t.Fatal("inlineManifest must survive an omitted member")
		}
		if it.ContentRating == nil || *it.ContentRating != contentrating.RatingMature {
			t.Fatalf("contentRating must survive an omitted member, got %v", it.ContentRating)
		}
		if it.ContentReasons == nil || len(*it.ContentReasons) != 1 {
			t.Fatalf("contentReasons must survive an omitted member, got %v", it.ContentReasons)
		}
	})

	t.Run("null document is a no-op", func(t *testing.T) {
		t.Parallel()
		it := PlaylistItem{Source: "https://old", Title: "Old Title"}
		if err := json.Unmarshal([]byte(`null`), &it); err != nil {
			t.Fatal(err)
		}
		if it.Source != "https://old" || it.Title != "Old Title" {
			t.Fatalf("null must leave the item untouched, got %+v", it)
		}
	})

	t.Run("present members replace, including clearing ones", func(t *testing.T) {
		t.Parallel()
		reasons := []string{"nudity"}
		it := PlaylistItem{
			Source:         "https://old",
			ContentRating:  ratingPtr(contentrating.RatingMature),
			ContentReasons: &reasons,
		}
		// A present null clears; so does a value that cannot be typed, on this tolerant path.
		if err := json.Unmarshal([]byte(`{"contentRating":null,"contentReasons":1}`), &it); err != nil {
			t.Fatal(err)
		}
		if it.ContentRating != nil {
			t.Fatalf("present null must clear the rating, got %q", *it.ContentRating)
		}
		if it.ContentReasons != nil {
			t.Fatalf("an untypeable present member must clear the reasons, got %v", *it.ContentReasons)
		}
		if it.Source != "https://old" {
			t.Fatalf("source must survive, got %q", it.Source)
		}
	})

	t.Run("reused slice element keeps its fields", func(t *testing.T) {
		t.Parallel()
		p := Playlist{Items: []PlaylistItem{{Source: "https://old", Title: "Kept"}}}
		if err := json.Unmarshal([]byte(`{"items":[{"source":"https://new"}]}`), &p); err != nil {
			t.Fatal(err)
		}
		if p.Items[0].Source != "https://new" || p.Items[0].Title != "Kept" {
			t.Fatalf("json reuses the backing array; got %+v", p.Items[0])
		}
	})
}
