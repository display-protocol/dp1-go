package dp1_test

import (
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/display-protocol/dp1-go"
	"github.com/display-protocol/dp1-go/extension/channels"
	"github.com/display-protocol/dp1-go/extension/contentrating"
	"github.com/display-protocol/dp1-go/extension/identity"
	"github.com/display-protocol/dp1-go/extension/playlists"
	"github.com/display-protocol/dp1-go/playlist"
	"github.com/display-protocol/dp1-go/playlistgroup"
	"github.com/display-protocol/dp1-go/refmanifest"
	"github.com/display-protocol/dp1-go/sign"
)

// --- ParseAndValidate: playlists ---

func TestParseAndValidatePlaylist_legacySigned(t *testing.T) {
	t.Parallel()
	pub, priv, _ := ed25519.GenerateKey(nil)
	core := playlist.Playlist{
		DPVersion: "1.0.0",
		Title:     "Hello",
		Items: []playlist.PlaylistItem{
			{Source: "https://example.com/work.html"},
		},
	}
	raw, err := json.Marshal(core)
	if err != nil {
		t.Fatal(err)
	}
	legacy, err := sign.SignLegacyEd25519(raw, priv)
	if err != nil {
		t.Fatal(err)
	}
	core.Signature = legacy
	signed, err := json.Marshal(core)
	if err != nil {
		t.Fatal(err)
	}
	p, err := dp1.ParseAndValidatePlaylist(signed)
	if err != nil {
		t.Fatal(err)
	}
	if p.Title != "Hello" {
		t.Fatal(p.Title)
	}
	if err := sign.VerifyLegacyEd25519(signed, p.Signature, pub); err != nil {
		t.Fatal(err)
	}
}

func TestParseAndValidatePlaylist_multiSigned(t *testing.T) {
	t.Parallel()
	_, priv, _ := ed25519.GenerateKey(nil)
	pl := playlist.Playlist{
		DPVersion: "1.1.0",
		Title:     "M",
		Items:     []playlist.PlaylistItem{{Source: "https://a"}},
	}
	body, _ := json.Marshal(pl)
	sig, err := sign.SignMultiEd25519(body, priv, playlist.RoleCurator, "2025-06-01T12:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	pl.Signatures = []playlist.Signature{sig}
	signed, _ := json.Marshal(pl)
	out, err := dp1.ParseAndValidatePlaylist(signed)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Signatures) != 1 {
		t.Fatal(out.Signatures)
	}
}

func TestParseAndValidatePlaylist_schemaRejectsInvalidDoc(t *testing.T) {
	t.Parallel()
	doc := []byte(`{
		"dpVersion":"1.1.0",
		"title":"",
		"items":[{"source":"https://a"}],
		"signatures":[{"alg":"ed25519","kid":"did:key:z6MkhaXgBZDvotDkL5257faiztiGiC2QtKLGpbnnEGta2doK",
			"ts":"2025-01-01T00:00:00Z",
			"payload_hash":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			"role":"curator","sig":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"}]
	}`)
	_, err := dp1.ParseAndValidatePlaylist(doc)
	if err == nil {
		t.Fatal("expected schema validation error")
	}
	assertValidationErrorChain(t, err)
}

func assertValidationErrorChain(t *testing.T, err error) {
	t.Helper()
	if errors.Is(err, dp1.ErrValidation) {
		return
	}
	var coded *dp1.CodedError
	if errors.As(err, &coded) && errors.Is(coded.Err, dp1.ErrValidation) {
		return
	}
	t.Fatalf("expected validation error (or CodedError wrapping it), got %T: %v", err, err)
}

func TestParseAndValidatePlaylistWithPlaylistsExtension(t *testing.T) {
	t.Parallel()
	_, priv, _ := ed25519.GenerateKey(nil)
	kid, err := sign.Ed25519DIDKey(priv.Public().(ed25519.PublicKey))
	if err != nil {
		t.Fatal(err)
	}
	dur := 15.0
	pl := playlist.Playlist{
		DPVersion: "1.1.0",
		Title:     "Ext",
		Note:      &playlists.Note{Text: "Welcome", Duration: &dur},
		Items: []playlist.PlaylistItem{{
			Source: "https://a",
			Note:   &playlists.Note{Text: "Track intro"},
		}},
		Summary: "A curated feed",
		Curators: []identity.Entity{
			{Name: "Alice", Key: kid},
		},
	}
	body, _ := json.Marshal(pl)
	sig, _ := sign.SignMultiEd25519(body, priv, playlist.RoleCurator, "2025-06-01T12:00:00Z")
	pl.Signatures = []playlist.Signature{sig}
	signed, _ := json.Marshal(pl)
	out, err := dp1.ParseAndValidatePlaylistWithPlaylistsExtension(signed)
	if err != nil {
		t.Fatal(err)
	}
	if out.Summary != "A curated feed" {
		t.Fatal(out.Summary)
	}
	if out.Note == nil || out.Note.Text != "Welcome" || out.Note.Duration == nil || *out.Note.Duration != 15 {
		t.Fatalf("note: %+v", out.Note)
	}
	if len(out.Items) != 1 || out.Items[0].Note == nil || out.Items[0].Note.Text != "Track intro" {
		t.Fatalf("item: %+v", out.Items)
	}
}

// TestParseAndValidatePlaylistWithPlaylistsExtension_openAtAndFullEntity locks in dp1#55
// end to end through the real embedded JSON Schema (not just the Go struct round trip
// playlist_test.go covers): openAt, playlist-level links, and curators carrying the full
// Entity shape (role/location/addresses/avatar/biographies/links) must all validate, not
// just decode. This is exactly the shape a real exhibition document, with curator and
// artist profiles, needs the feed to accept.
func TestParseAndValidatePlaylistWithPlaylistsExtension_openAtAndFullEntity(t *testing.T) {
	t.Parallel()
	_, priv, _ := ed25519.GenerateKey(nil)
	kid, err := sign.Ed25519DIDKey(priv.Public().(ed25519.PublicKey))
	if err != nil {
		t.Fatal(err)
	}
	pl := playlist.Playlist{
		DPVersion: "1.1.0",
		Title:     "Ext",
		Items:     []playlist.PlaylistItem{{Source: "https://a"}},
		OpenAt:    "2026-10-01T18:00:00-04:00",
		Curators: []identity.Entity{{
			Name:        "Alice",
			Key:         kid,
			Role:        "curator",
			Location:    "Brooklyn, USA",
			Addresses:   []string{"0xabc"},
			Avatar:      &identity.Thumbnail{URI: "https://a/avatar.png"},
			Biographies: []identity.Biography{{Text: "Curator bio"}},
			Links:       []identity.Link{{Type: identity.LinkTypeWebsite, URL: "https://alice.example"}},
		}},
		Links: []identity.Link{{
			Type:  identity.LinkTypeArticle,
			URL:   "https://press.example/review",
			Title: "A review",
		}},
	}
	body, _ := json.Marshal(pl)
	sig, _ := sign.SignMultiEd25519(body, priv, playlist.RoleCurator, "2026-10-01T12:00:00Z")
	pl.Signatures = []playlist.Signature{sig}
	signed, _ := json.Marshal(pl)
	out, err := dp1.ParseAndValidatePlaylistWithPlaylistsExtension(signed)
	if err != nil {
		t.Fatal(err)
	}
	if out.OpenAt != pl.OpenAt {
		t.Fatalf("openAt: got %q want %q", out.OpenAt, pl.OpenAt)
	}
	if len(out.Curators) != 1 || out.Curators[0].Role != "curator" || out.Curators[0].Location != "Brooklyn, USA" {
		t.Fatalf("curator: %+v", out.Curators)
	}
	if len(out.Links) != 1 || out.Links[0].Title != "A review" {
		t.Fatalf("links: %+v", out.Links)
	}
}

// TestParseAndValidatePlaylist_seriesIDUnderflowDoesNotExpand drives the amplification
// through the primary documented entrypoint, because schema validation does NOT stop it.
//
// encoding/json is asymmetric on out-of-range numbers when decoding into `any`, which is
// what internal/validate does: an overflowing literal (1e400) errors, but an underflowing
// one (1e-1000000) becomes float64 0 with no error at all. The schema therefore sees a
// well-formed integer seriesId of 0 and passes the document, and ParseAndValidatePlaylist
// then unmarshals the ORIGINAL bytes into the Go types — where the raw literal reaches
// SeriesID's guard. Only that guard stands between a ten-byte exponent and big.Rat
// building a million-digit denominator.
//
// The measurement is a DIFFERENCE against a control document, not an absolute ceiling:
// parsing and schema-validating a playlist at all costs about 1.4 MB here, which would
// swamp any fixed threshold and make the test pass whether or not the guard ran. The
// control is the same document with an ordinary seriesId, so what is left is the cost the
// literal itself adds — roughly 2.9 MB unguarded, nothing guarded.
//
// An error is not asserted as the proof for the same reason as the unit tests in
// playlist/: the literal is fractional, so IsInt would refuse it either way.
func TestParseAndValidatePlaylist_seriesIDUnderflowDoesNotExpand(t *testing.T) {
	t.Parallel()
	// Comfortably under the ~2.9 MB the unguarded expansion costs, comfortably over the
	// handful of KB that parsing one extra literal costs.
	const maxExtraAlloc = 256 << 10
	doc := func(seriesID string) []byte {
		return []byte(fmt.Sprintf(`{
			"dpVersion": "1.1.0",
			"title": "Underflow",
			"items": [{
				"source": "https://a",
				"provenance": {"type": "onChain", "contract": {"chain": "evm", "seriesId": %s}}
			}],
			"signatures": [%s]
		}`, seriesID, dummySignatureJSON))
	}
	cost := func(d []byte) (uint64, error) {
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		_, err := dp1.ParseAndValidatePlaylist(d)
		runtime.ReadMemStats(&after)
		return after.TotalAlloc - before.TotalAlloc, err
	}

	// Warm every lazily-built schema before measuring, so the control is not charged for
	// one-time setup the attack document would then appear to avoid.
	if _, err := cost(doc(`512`)); err != nil {
		t.Fatalf("control document must validate: %v", err)
	}
	control, err := cost(doc(`512`))
	if err != nil {
		t.Fatalf("control document must validate: %v", err)
	}
	attack, err := cost(doc(`1e-1000000`))
	if err == nil {
		t.Fatal("ParseAndValidatePlaylist accepted an underflowing seriesId, want an error")
	}
	if attack > control+maxExtraAlloc {
		t.Fatalf("the underflowing literal added %d bytes over the control (%d vs %d), want under %d: big.Rat expanded it",
			attack-control, attack, control, maxExtraAlloc)
	}
}

// TestParseAndValidatePlaylist_imageRenderingAndSeriesIDThroughSchema drives the two
// dp1#55 core-schema fields through the real embedded schema rather than through Go
// types alone, in both of seriesId's wire forms.
//
// The Go-level tests in playlist/ cannot stand in for this: ImageRendering is a plain
// string, so only the schema's enum constrains it, and SeriesID.UnmarshalJSON only ever
// sees a value the schema has already let through — validation runs before decode. A
// schema edit that dropped either constraint would leave every one of those tests green.
//
// It uses dummySignatureJSON rather than signing: a raw document is the only way to put
// an integer seriesId on the wire, since SeriesID always marshals as a string.
func TestParseAndValidatePlaylist_imageRenderingAndSeriesIDThroughSchema(t *testing.T) {
	t.Parallel()
	// 2^256-1, well past float64's exact range — the reason the string form exists.
	const bigSeries = "115792089237316195423570985008687907853269984665640564039457584007913129639935"
	cases := []struct {
		name     string
		seriesID string // raw JSON, so an unquoted integer is expressible
		want     playlist.SeriesID
	}{
		{"integer wire form", `512`, "512"},
		{"decimal string wire form", `"` + bigSeries + `"`, playlist.SeriesID(bigSeries)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			doc := []byte(fmt.Sprintf(`{
				"dpVersion": "1.1.0",
				"title": "Pixel art",
				"items": [{
					"source": "https://a",
					"display": {"imageRendering": "pixelated"},
					"provenance": {"type": "onChain", "contract": {"chain": "evm", "seriesId": %s}}
				}],
				"signatures": [%s]
			}`, tc.seriesID, dummySignatureJSON))
			out, err := dp1.ParseAndValidatePlaylist(doc)
			if err != nil {
				t.Fatal(err)
			}
			if out.Items[0].Display == nil || out.Items[0].Display.ImageRendering != "pixelated" {
				t.Fatalf("imageRendering: %+v", out.Items[0].Display)
			}
			got := out.Items[0].Provenance.Contract.SeriesID
			if got == nil || *got != tc.want {
				t.Fatalf("seriesId: got %v want %q", got, tc.want)
			}
			// Re-marshaling normalizes the integer form to a string; the schema's oneOf
			// accepts both, so the document this SDK emits must still validate.
			round, err := json.Marshal(out)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := dp1.ParseAndValidatePlaylist(round); err != nil {
				t.Fatalf("re-marshaled playlist no longer validates: %v", err)
			}
		})
	}
}

func TestParseAndValidatePlaylistWithContentRatingExtension(t *testing.T) {
	t.Parallel()
	_, priv, _ := ed25519.GenerateKey(nil)
	rating := contentrating.RatingMature
	reasons := []string{"nudity"}
	pl := playlist.Playlist{
		DPVersion: "1.1.0",
		Title:     "Rated",
		Items: []playlist.PlaylistItem{{
			Source:         "https://a",
			ContentRating:  &rating,
			ContentReasons: &reasons,
		}},
	}
	body, _ := json.Marshal(pl)
	sig, _ := sign.SignMultiEd25519(body, priv, playlist.RoleCurator, "2025-06-01T12:00:00Z")
	pl.Signatures = []playlist.Signature{sig}
	signed, _ := json.Marshal(pl)
	out, err := dp1.ParseAndValidatePlaylistWithPlaylistsAndContentRatingExtensions(signed)
	if err != nil {
		t.Fatal(err)
	}
	if out.Items[0].ContentRating == nil || *out.Items[0].ContentRating != contentrating.RatingMature {
		t.Fatalf("rating: %+v", out.Items[0].ContentRating)
	}
	// The core + content-rating entrypoint must accept the same document: the playlists overlay
	// is optional, so a producer that ships ratings without it still parses.
	coreOnly, err := dp1.ParseAndValidatePlaylistWithContentRatingExtension(signed)
	if err != nil {
		t.Fatal(err)
	}
	if coreOnly.Items[0].ContentRating == nil || *coreOnly.Items[0].ContentRating != contentrating.RatingMature {
		t.Fatalf("rating (core+content-rating): %+v", coreOnly.Items[0].ContentRating)
	}
	if coreOnly.Items[0].ContentReasons == nil || len(*coreOnly.Items[0].ContentReasons) != 1 {
		t.Fatalf("reasons: %+v", coreOnly.Items[0].ContentReasons)
	}
	if err := sign.VerifyMultiSignature(signed, out.Signatures[0]); err != nil {
		t.Fatal(err)
	}
	tampered := []byte(strings.Replace(string(signed), `"contentRating":"mature"`, `"contentRating":"general"`, 1))
	if err := sign.VerifyMultiSignature(tampered, out.Signatures[0]); err == nil {
		t.Fatalf("rating change must invalidate signature: %v", err)
	}
	malformed := []byte(strings.Replace(string(signed), `"contentRating":"mature"`, `"contentRating":null`, 1))
	if _, err := dp1.ParseAndValidatePlaylistWithContentRatingExtension(malformed); err == nil {
		t.Fatal("expected present null contentRating to fail")
	}
	if _, err := dp1.ParseAndValidatePlaylistWithPlaylistsAndContentRatingExtensions(malformed); err == nil {
		t.Fatal("expected present null contentRating to fail with both overlays")
	}
}

// Core DP-1 and the playlists extension both permit item properties they do not describe, so a
// consumer that never opted into the draft content-rating extension must still be able to read a
// playlist carrying one — including a malformed or newer representation it cannot type. Without
// the lenient decode in PlaylistItem.UnmarshalJSON, the typed fields would fail json.Unmarshal on
// a document the selected schema had just accepted.
func TestContentRatingToleratedByNonAwareParsers(t *testing.T) {
	t.Parallel()
	const sigBlock = `"signatures":[{"alg":"ed25519","kid":"did:key:z6MkhaXgBZDvotDkL5257faiztiGiC2QtKLGpbnnEGta2doK","ts":"2025-01-01T00:00:00Z","payload_hash":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","role":"curator","sig":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"}]`

	for _, tc := range []struct {
		name string
		item string
		// wantRating is the rating the non-aware parsers should surface; "" means nil, i.e. the
		// same "absent, therefore unrated" state seen before the extension existed.
		wantRating contentrating.Rating
	}{
		{"rating_wrong_type", `"source":"https://a","contentRating":1`, ""},
		{"rating_future_value", `"source":"https://a","contentRating":"adults-only"`, "adults-only"},
		{"reasons_wrong_type", `"source":"https://a","contentReasons":"nudity"`, ""},
		{"reasons_wrong_element_type", `"source":"https://a","contentReasons":[1,2]`, ""},
		{"rating_null", `"source":"https://a","contentRating":null`, ""},
		{"well_formed_still_decodes", `"source":"https://a","contentRating":"mature"`, contentrating.RatingMature},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			doc := []byte(`{"dpVersion":"1.1.0","title":"x","items":[{` + tc.item + `}],` + sigBlock + `}`)

			core, err := dp1.ParseAndValidatePlaylist(doc)
			if err != nil {
				t.Fatalf("core parser must accept the document: %v", err)
			}
			withPlaylists, err := dp1.ParseAndValidatePlaylistWithPlaylistsExtension(doc)
			if err != nil {
				t.Fatalf("playlists-only parser must accept the document: %v", err)
			}

			for _, got := range []*playlist.Playlist{core, withPlaylists} {
				switch {
				case tc.wantRating == "":
					if got.Items[0].ContentRating != nil {
						t.Fatalf("want nil rating, got %q", *got.Items[0].ContentRating)
					}
				case got.Items[0].ContentRating == nil:
					t.Fatalf("want rating %q, got nil", tc.wantRating)
				case *got.Items[0].ContentRating != tc.wantRating:
					t.Fatalf("want rating %q, got %q", tc.wantRating, *got.Items[0].ContentRating)
				}
			}
		})
	}
}

// The leniency above must not reach the parsers that do implement the extension: those validate
// against the schema before decoding, so a rating of the wrong JSON *type* is rejected there
// rather than silently flattened to nil.
//
// An unrecognized rating *string* is a separate matter and is not listed here: the wire
// vocabulary is open (§3.3), so a value this SDK does not define is valid and is treated as
// unrated. TestUnknownRatingIsAcceptedEverywhere covers it.
func TestContentRatingLeniencyDoesNotReachAwareParsers(t *testing.T) {
	t.Parallel()
	const sigBlock = `"signatures":[{"alg":"ed25519","kid":"did:key:z6MkhaXgBZDvotDkL5257faiztiGiC2QtKLGpbnnEGta2doK","ts":"2025-01-01T00:00:00Z","payload_hash":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","role":"curator","sig":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"}]`
	for _, item := range []string{
		`"source":"https://a","contentRating":1`,
		`"source":"https://a","contentRating":true`,
		`"source":"https://a","contentReasons":"nudity"`,
		`"source":"https://a","contentReasons":[1,2]`,
		`"source":"https://a","contentRating":null`,
	} {
		doc := []byte(`{"dpVersion":"1.1.0","title":"x","items":[{` + item + `}],` + sigBlock + `}`)
		if _, err := dp1.ParseAndValidatePlaylistWithContentRatingExtension(doc); err == nil {
			t.Fatalf("content-rating-aware parser must reject %s", item)
		}
		if _, err := dp1.ParseAndValidatePlaylistWithPlaylistsAndContentRatingExtensions(doc); err == nil {
			t.Fatalf("combined parser must reject %s", item)
		}
	}
}

// Product decision recorded in display-protocol/dp1#52 (§3.3): the wire vocabulary is open, so a
// rating string this SDK version does not define is valid everywhere, decodes, and means unrated.
// Nothing is assumed from a label the consumer does not know — only "mature" hides anything — so
// a future vocabulary must never cost an item its playability or strand it at a parser.
func TestUnknownRatingIsAcceptedEverywhere(t *testing.T) {
	t.Parallel()
	const sigBlock = `"signatures":[{"alg":"ed25519","kid":"did:key:z6MkhaXgBZDvotDkL5257faiztiGiC2QtKLGpbnnEGta2doK","ts":"2025-01-01T00:00:00Z","payload_hash":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","role":"curator","sig":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"}]`
	for _, unknown := range []string{"adults-only", "teen", "", "GENERAL"} {
		doc := []byte(`{"dpVersion":"1.1.0","title":"x","items":[{"source":"https://a","contentRating":` +
			strconv.Quote(unknown) + `}],` + sigBlock + `}`)

		for name, parse := range map[string]func([]byte) (*playlist.Playlist, error){
			"core":                     dp1.ParseAndValidatePlaylist,
			"playlists":                dp1.ParseAndValidatePlaylistWithPlaylistsExtension,
			"content-rating":           dp1.ParseAndValidatePlaylistWithContentRatingExtension,
			"playlists+content-rating": dp1.ParseAndValidatePlaylistWithPlaylistsAndContentRatingExtensions,
		} {
			out, err := parse(doc)
			if err != nil {
				t.Fatalf("%s parser must accept rating %q: %v", name, unknown, err)
			}
			got := out.Items[0].ContentRating
			if got == nil {
				t.Fatalf("%s parser: rating %q must decode, got nil", name, unknown)
			}
			if string(*got) != unknown {
				t.Fatalf("%s parser: want rating %q, got %q", name, unknown, *got)
			}
			if got.Known() {
				t.Fatalf("%s parser: rating %q must not report as Known", name, unknown)
			}
		}

		// The fragment validator is the ingestion boundary and must agree.
		if err := dp1.ValidateContentRatingExtension([]byte(`{"items":[{"contentRating":` +
			strconv.Quote(unknown) + `}]}`)); err != nil {
			t.Fatalf("fragment validator must accept rating %q: %v", unknown, err)
		}
	}
}

// An unknown rating must survive decode and re-encode byte-identically. A consumer that merely
// relays a document re-encodes it, and dropping or rewriting the label there would change the JCS
// payload and break the signature over it.
func TestUnknownRatingRoundTrip(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{
		`{"source":"https://a","contentRating":"adults-only"}`,
		`{"source":"https://a","contentRating":"teen","contentReasons":["language"]}`,
		`{"source":"https://a","contentRating":""}`,
	} {
		var item playlist.PlaylistItem
		if err := json.Unmarshal([]byte(raw), &item); err != nil {
			t.Fatalf("%s: %v", raw, err)
		}
		got, err := json.Marshal(item)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != raw {
			t.Fatalf("round trip changed the document:\n got %s\nwant %s", got, raw)
		}
	}
}

func TestContentReasonsPresenceRoundTrip(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		raw  string
		want string
	}{
		{"absent", `{"source":"https://a"}`, `{"source":"https://a"}`},
		{"present_empty", `{"source":"https://a","contentReasons":[]}`, `{"source":"https://a","contentReasons":[]}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var item playlist.PlaylistItem
			if err := json.Unmarshal([]byte(tc.raw), &item); err != nil {
				t.Fatal(err)
			}
			got, err := json.Marshal(item)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tc.want {
				t.Fatalf("got %s, want %s", got, tc.want)
			}
		})
	}
}

func TestValidateContentRatingExtensionFragment(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{
		`{"items":[{"contentRating":"general","contentReasons":[]}]}`,
		// Open vocabulary (§3.3): an unrecognized rating string is valid, and means unrated.
		`{"items":[{"contentRating":"unknown"}]}`,
	} {
		if err := dp1.ValidateContentRatingExtension([]byte(raw)); err != nil {
			t.Fatalf("expected valid fragment %s: %v", raw, err)
		}
	}
	for _, raw := range []string{
		`{"items":[{"contentRating":null}]}`,
		`{"items":[{"contentRating":1}]}`,
		`{"items":[{"contentReasons":[""]}]}`,
	} {
		if err := dp1.ValidateContentRatingExtension([]byte(raw)); err == nil {
			t.Fatalf("expected invalid fragment: %s", raw)
		}
	}
}

// --- ParseAndValidate: playlist-group, ref manifest, channel ---

func TestParseAndValidatePlaylistGroup(t *testing.T) {
	t.Parallel()
	_, priv, _ := ed25519.GenerateKey(nil)
	g := playlistgroup.Group{
		ID:        "385f79b6-a45f-4c1c-8080-e93a192adccc",
		Title:     "Ex",
		Playlists: []string{"https://feed.example/p.json"},
		Created:   "2025-06-01T12:00:00Z",
	}
	body, _ := json.Marshal(g)
	sig, _ := sign.SignMultiEd25519(body, priv, playlist.RoleCurator, "2025-06-01T12:00:00Z")
	g.Signatures = []playlist.Signature{sig}
	signed, _ := json.Marshal(g)
	out, err := dp1.ParseAndValidatePlaylistGroup(signed)
	if err != nil {
		t.Fatal(err)
	}
	if out.Title != "Ex" {
		t.Fatal(out.Title)
	}
}

func TestParseAndValidateRefManifest(t *testing.T) {
	t.Parallel()
	m := refmanifest.Manifest{
		RefVersion: "0.1.0",
		ID:         "ref-1",
		Created:    "2025-06-01T12:00:00Z",
		Locale:     "en",
		Metadata: &refmanifest.Metadata{
			Title: "Work",
			Artists: []refmanifest.Artist{
				{Name: "A"},
			},
			Thumbnails: map[string]refmanifest.Thumbnail{
				"default": {URI: "ipfs://bafy", W: intPtr(100), H: intPtr(100)},
			},
		},
	}
	data, _ := json.Marshal(m)
	out, err := dp1.ParseAndValidateRefManifest(data)
	if err != nil {
		t.Fatal(err)
	}
	if out.Metadata.Title != "Work" {
		t.Fatal(out.Metadata.Title)
	}
}

// TestParseAndValidateRefManifest_artistCarriesFullEntity is the round-trip half of the
// Entity consolidation: the ref-manifest schema's Artist $def is a $ref to $defs/Entity,
// so an artist may carry key/role/location and its links[] may carry
// title/date/thumbnail/authors. Validation alone proves nothing here — the regression
// this locks in is the decode step dropping fields the schema had just accepted, which a
// test that only checks err == nil would pass while the data silently vanished. So the
// assertions read the fields back off the decoded manifest.
func TestParseAndValidateRefManifest_artistCarriesFullEntity(t *testing.T) {
	t.Parallel()
	doc := []byte(`{
		"refVersion": "0.1.0",
		"id": "ref-entity",
		"created": "2025-06-01T12:00:00Z",
		"locale": "en",
		"metadata": {
			"title": "Work",
			"artists": [{
				"name": "A",
				"key": "did:key:z6MkArtist",
				"role": "artist",
				"location": "Lisbon, Portugal",
				"links": [{
					"type": "article",
					"url": "https://press.example/piece",
					"title": "On the work",
					"date": "2026-02-01T00:00:00Z",
					"thumbnail": {"uri": "https://press.example/cover.jpg"},
					"authors": [{"name": "A Critic", "role": "other"}]
				}]
			}]
		}
	}`)
	out, err := dp1.ParseAndValidateRefManifest(doc)
	if err != nil {
		t.Fatal(err)
	}
	a := out.Metadata.Artists[0]
	if a.Key != "did:key:z6MkArtist" || a.Role != "artist" || a.Location != "Lisbon, Portugal" {
		t.Fatalf("artist entity fields dropped on decode: %+v", a)
	}
	l := a.Links[0]
	if l.Title != "On the work" || l.Date != "2026-02-01T00:00:00Z" {
		t.Fatalf("link title/date dropped on decode: %+v", l)
	}
	if l.Thumbnail == nil || l.Thumbnail.URI != "https://press.example/cover.jpg" {
		t.Fatalf("link thumbnail dropped on decode: %+v", l.Thumbnail)
	}
	if len(l.Authors) != 1 || l.Authors[0].Name != "A Critic" {
		t.Fatalf("link authors dropped on decode: %+v", l.Authors)
	}
	// Re-marshaling must still validate: a field decoded but not re-emitted is the same
	// data loss one step later.
	round, err := json.Marshal(out)
	if err != nil {
		t.Fatal(err)
	}
	back, err := dp1.ParseAndValidateRefManifest(round)
	if err != nil {
		t.Fatalf("re-marshaled manifest no longer validates: %v", err)
	}
	if back.Metadata.Artists[0].Location != "Lisbon, Portugal" {
		t.Fatalf("location lost on re-marshal: %+v", back.Metadata.Artists[0])
	}
}

func intPtr(i int) *int { return &i }

// dummySignatureJSON is schema-valid but cryptographically meaningless. ParseAndValidate*
// only runs JSON Schema, so tests that target schema shape use it instead of signing —
// keeping the assertion on the field under test rather than on signature plumbing.
const dummySignatureJSON = `{"alg":"ed25519","kid":"did:key:z6MkhaXgBZDvotDkL5257faiztiGiC2QtKLGpbnnEGta2doK",
	"ts":"2025-01-01T00:00:00Z",
	"payload_hash":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	"role":"curator","sig":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"}`

func dummySignature(t *testing.T) playlist.Signature {
	t.Helper()
	var s playlist.Signature
	if err := json.Unmarshal([]byte(dummySignatureJSON), &s); err != nil {
		t.Fatal(err)
	}
	return s
}

// Thumbnail.w / Thumbnail.h were dropped from the ref-manifest required list
// (core changelog 2026-08-12): a bare thumbnail URL must validate on its own.
func TestParseAndValidateRefManifest_thumbnailWithoutDimensions(t *testing.T) {
	t.Parallel()
	m := refmanifest.Manifest{
		RefVersion: "0.1.0",
		ID:         "ref-1",
		Created:    "2025-06-01T12:00:00Z",
		Locale:     "en",
		Metadata: &refmanifest.Metadata{
			Thumbnails: map[string]refmanifest.Thumbnail{
				"default": {URI: "https://example.com/thumb.png"},
			},
		},
	}
	data, _ := json.Marshal(m)
	if strings.Contains(string(data), `"w"`) || strings.Contains(string(data), `"h"`) {
		t.Fatalf("absent dimensions must not marshal: %s", data)
	}
	out, err := dp1.ParseAndValidateRefManifest(data)
	if err != nil {
		t.Fatal(err)
	}
	th := out.Metadata.Thumbnails["default"]
	if th.W != nil || th.H != nil {
		t.Fatalf("expected nil dimensions, got w=%v h=%v", th.W, th.H)
	}
}

func TestParseAndValidateRefManifest_rejectsZeroThumbnailWidth(t *testing.T) {
	t.Parallel()
	// w stays constrained when present (minimum 1); only the requirement was relaxed.
	doc := []byte(`{"refVersion":"0.1.0","id":"ref-1","created":"2025-06-01T12:00:00Z","locale":"en",
		"metadata":{"thumbnails":{"default":{"uri":"https://example.com/t.png","w":0,"h":10}}}}`)
	if _, err := dp1.ParseAndValidateRefManifest(doc); err == nil {
		t.Fatal("expected validation error for w=0")
	}
}

// Playlists extension §3.6: an item may carry a full ref manifest inline, validated by the
// unmodified ref-manifest schema.
func TestParseAndValidatePlaylistWithPlaylistsExtension_inlineManifest(t *testing.T) {
	t.Parallel()
	pl := playlist.Playlist{
		DPVersion: "1.1.0",
		Title:     "Inline",
		Items: []playlist.PlaylistItem{{
			Source: "https://example.com/work.html",
			InlineManifest: mustMarshal(t, &refmanifest.Manifest{
				RefVersion: "0.1.0",
				ID:         "ref-9d26ecb3",
				Created:    "2026-07-28T00:00:00Z",
				Locale:     "en",
				Metadata: &refmanifest.Metadata{
					Title:   "Pre-Process",
					Artists: []refmanifest.Artist{{Name: "Casey Reas"}},
					Thumbnails: map[string]refmanifest.Thumbnail{
						"default": {URI: "https://example.com/thumb.png"},
					},
				},
			}),
		}},
		Signatures: []playlist.Signature{dummySignature(t)},
	}
	data, _ := json.Marshal(pl)
	out, err := dp1.ParseAndValidatePlaylistWithPlaylistsExtension(data)
	if err != nil {
		t.Fatal(err)
	}
	// Schema-valid on this path, so the decode cannot fail.
	im, err := out.Items[0].ParseInlineManifest()
	if err != nil {
		t.Fatal(err)
	}
	if im == nil || im.ID != "ref-9d26ecb3" || im.Metadata.Title != "Pre-Process" {
		t.Fatalf("inlineManifest: %+v", im)
	}
}

func mustMarshal(t *testing.T, v any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestParseAndValidatePlaylistWithPlaylistsExtension_rejectsMalformedInlineManifest(t *testing.T) {
	t.Parallel()
	// Missing the required `locale` envelope field: invalid exactly as a fetched manifest would be.
	doc := []byte(`{"dpVersion":"1.1.0","title":"Inline","items":[{"source":"https://a",
		"inlineManifest":{"refVersion":"0.1.0","id":"ref-1","created":"2026-07-28T00:00:00Z"}}],
		"signatures":[` + dummySignatureJSON + `]}`)
	_, err := dp1.ParseAndValidatePlaylistWithPlaylistsExtension(doc)
	if err == nil {
		t.Fatal("expected validation error for malformed inlineManifest")
	}
	assertValidationErrorChain(t, err)
}

// Core DP-1 tolerates unknown fields, so a schema-invalid inlineManifest must pass the
// core-only path untouched: §3.6 says a player that ignores ref manifests may ignore it.
func TestParseAndValidatePlaylist_coreIgnoresInlineManifest(t *testing.T) {
	t.Parallel()
	doc := []byte(`{"dpVersion":"1.1.0","title":"Inline","items":[{"source":"https://a",
		"inlineManifest":{"refVersion":"0.1.0","id":"ref-1","created":"2026-07-28T00:00:00Z"}}],
		"signatures":[` + dummySignatureJSON + `]}`)
	if _, err := dp1.ParseAndValidatePlaylist(doc); err != nil {
		t.Fatal(err)
	}
}

// …and the tolerance has to survive the decode step too, which is why the field is raw JSON.
// A core-only player must be able to parse a playlist carrying an inlineManifest of any shape,
// including one no manifest schema would accept: with a typed field these documents failed
// json.Unmarshal after the core schema had already passed them.
func TestParseAndValidatePlaylist_coreToleratesTypeMismatchedInlineManifest(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"inline_manifest_is_a_string": `"https://m.example/x.json"`,
		"inline_manifest_is_an_array": `[]`,
		"thumbnail_width_is_a_string": `{"refVersion":"0.1.0","id":"r","created":"2026-07-28T00:00:00Z","locale":"en",
			"metadata":{"thumbnails":{"default":{"uri":"https://m.example/t.png","w":"1200"}}}}`,
	}
	for name, manifest := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			doc := []byte(`{"dpVersion":"1.1.0","title":"Inline","items":[{"source":"https://a",
				"inlineManifest":` + manifest + `}],
				"signatures":[` + dummySignatureJSON + `]}`)
			// The core schema is happy — it describes no extension field.
			if err := dp1.PlaylistCoreSchemaValidate(doc); err != nil {
				t.Fatalf("core schema should tolerate the unknown field: %v", err)
			}
			p, err := dp1.ParseAndValidatePlaylist(doc)
			if err != nil {
				t.Fatalf("core parse must tolerate it too: %v", err)
			}
			// The bytes are kept verbatim, and the error surfaces only when someone asks for a
			// manifest — the point at which a caller has opted into the extension.
			if _, err := p.Items[0].ParseInlineManifest(); err == nil {
				t.Fatal("expected the decode error at ParseInlineManifest")
			}
			if _, err := dp1.ParseAndValidateRefManifest(p.Items[0].InlineManifest); err == nil {
				t.Fatal("expected raw bytes to be rejected by the ref-manifest validator")
			}
		})
	}
}

func TestParseAndValidateChannel(t *testing.T) {
	t.Parallel()
	_, priv, _ := ed25519.GenerateKey(nil)
	ch := channels.Channel{
		ID:        "385f79b6-a45f-4c1c-8080-e93a192adccc",
		Slug:      "chan",
		Title:     "Ch",
		Version:   "1.0.0",
		Created:   "2025-06-01T12:00:00Z",
		Playlists: []string{"https://feed.example/p.json"},
	}
	body, _ := json.Marshal(ch)
	sig, _ := sign.SignMultiEd25519(body, priv, playlist.RoleFeed, "2025-06-01T12:00:00Z")
	ch.Signatures = []playlist.Signature{sig}
	signed, _ := json.Marshal(ch)
	out, err := dp1.ParseAndValidateChannel(signed)
	if err != nil {
		t.Fatal(err)
	}
	if out.Title != "Ch" {
		t.Fatal(out.Title)
	}
}

// --- Decode errors (schema hooks; sequential subtests — not parallel with globals) ---

func TestParseAndValidate_decodeErrors(t *testing.T) {
	cases := []struct {
		name string
		run  func(t *testing.T)
	}{
		{"playlist", func(t *testing.T) {
			orig := dp1.PlaylistCoreSchemaValidate
			dp1.PlaylistCoreSchemaValidate = func([]byte) error { return nil }
			t.Cleanup(func() { dp1.PlaylistCoreSchemaValidate = orig })
			_, err := dp1.ParseAndValidatePlaylist([]byte(`{"dpVersion":"1.1.0","title":"x","items":[{"source":true}],"signature":"ed25519:aa"}`))
			if err == nil || !strings.Contains(err.Error(), "decode playlist") {
				t.Fatalf("got %v", err)
			}
			var coded *dp1.CodedError
			if !errors.As(err, &coded) || coded.Code != dp1.CodePlaylistInvalid {
				t.Fatalf("expected *CodedError with CodePlaylistInvalid, got %v", err)
			}
		}},
		{"playlist_ext", func(t *testing.T) {
			orig := dp1.PlaylistWithPlaylistsExtensionSchemaValidate
			dp1.PlaylistWithPlaylistsExtensionSchemaValidate = func([]byte) error { return nil }
			t.Cleanup(func() { dp1.PlaylistWithPlaylistsExtensionSchemaValidate = orig })
			_, err := dp1.ParseAndValidatePlaylistWithPlaylistsExtension([]byte(`{"dpVersion":"1.1.0","title":"x","items":[{"source":true}],"signatures":[]}`))
			if err == nil || !strings.Contains(err.Error(), "decode playlist") {
				t.Fatalf("got %v", err)
			}
			var coded *dp1.CodedError
			if !errors.As(err, &coded) || coded.Code != dp1.CodePlaylistInvalid {
				t.Fatalf("expected CodePlaylistInvalid, got %v", err)
			}
		}},
		{"playlist_content_rating", func(t *testing.T) {
			orig := dp1.PlaylistWithContentRatingExtensionSchemaValidate
			dp1.PlaylistWithContentRatingExtensionSchemaValidate = func([]byte) error { return nil }
			t.Cleanup(func() { dp1.PlaylistWithContentRatingExtensionSchemaValidate = orig })
			// A core field, not a rating: content-rating members decode leniently by design
			// (see PlaylistItem.UnmarshalJSON), so they cannot exercise the decode-error path.
			_, err := dp1.ParseAndValidatePlaylistWithContentRatingExtension([]byte(`{"dpVersion":"1.1.0","title":"x","items":[{"source":true,"contentRating":"mature"}],"signatures":[]}`))
			if err == nil || !strings.Contains(err.Error(), "decode playlist") {
				t.Fatalf("got %v", err)
			}
			var coded *dp1.CodedError
			if !errors.As(err, &coded) || coded.Code != dp1.CodePlaylistInvalid {
				t.Fatalf("expected CodePlaylistInvalid, got %v", err)
			}
		}},
		{"playlist_playlists_and_content_rating", func(t *testing.T) {
			orig := dp1.PlaylistWithPlaylistsAndContentRatingExtensionsSchemaValidate
			dp1.PlaylistWithPlaylistsAndContentRatingExtensionsSchemaValidate = func([]byte) error { return nil }
			t.Cleanup(func() { dp1.PlaylistWithPlaylistsAndContentRatingExtensionsSchemaValidate = orig })
			_, err := dp1.ParseAndValidatePlaylistWithPlaylistsAndContentRatingExtensions([]byte(`{"dpVersion":"1.1.0","title":"x","items":[{"source":true,"contentReasons":["nudity"]}],"signatures":[]}`))
			if err == nil || !strings.Contains(err.Error(), "decode playlist") {
				t.Fatalf("got %v", err)
			}
			var coded *dp1.CodedError
			if !errors.As(err, &coded) || coded.Code != dp1.CodePlaylistInvalid {
				t.Fatalf("expected CodePlaylistInvalid, got %v", err)
			}
		}},
		{"group", func(t *testing.T) {
			orig := dp1.PlaylistGroupSchemaValidate
			dp1.PlaylistGroupSchemaValidate = func([]byte) error { return nil }
			t.Cleanup(func() { dp1.PlaylistGroupSchemaValidate = orig })
			_, err := dp1.ParseAndValidatePlaylistGroup([]byte(`{"id":"385f79b6-a45f-4c1c-8080-e93a192adccc","title":1,"playlists":[],"created":"2025-01-01T00:00:00Z","signature":"ed25519:aa"}`))
			if err == nil || !strings.Contains(err.Error(), "decode playlist-group") {
				t.Fatalf("got %v", err)
			}
			var coded *dp1.CodedError
			if !errors.As(err, &coded) || coded.Code != dp1.CodePlaylistGroupInvalid {
				t.Fatalf("expected CodePlaylistGroupInvalid, got %v", err)
			}
		}},
		{"ref", func(t *testing.T) {
			orig := dp1.RefManifestSchemaValidate
			dp1.RefManifestSchemaValidate = func([]byte) error { return nil }
			t.Cleanup(func() { dp1.RefManifestSchemaValidate = orig })
			_, err := dp1.ParseAndValidateRefManifest([]byte(`{"refVersion":"0.1.0","id":1,"created":"2025-01-01T00:00:00Z","locale":"en"}`))
			if err == nil || !strings.Contains(err.Error(), "decode ref manifest") {
				t.Fatalf("got %v", err)
			}
			var coded *dp1.CodedError
			if !errors.As(err, &coded) || coded.Code != dp1.CodeRefManifestInvalid {
				t.Fatalf("expected CodeRefManifestInvalid, got %v", err)
			}
		}},
		{"channel", func(t *testing.T) {
			orig := dp1.ChannelExtensionSchemaValidate
			dp1.ChannelExtensionSchemaValidate = func([]byte) error { return nil }
			t.Cleanup(func() { dp1.ChannelExtensionSchemaValidate = orig })
			_, err := dp1.ParseAndValidateChannel([]byte(`{"id":"385f79b6-a45f-4c1c-8080-e93a192adccc","slug":"s","title":1,"version":"1.0.0","created":"2025-01-01T00:00:00Z","playlists":[],"signature":"ed25519:aa"}`))
			if err == nil || !strings.Contains(err.Error(), "decode channel") {
				t.Fatalf("got %v", err)
			}
			var coded *dp1.CodedError
			if !errors.As(err, &coded) || coded.Code != dp1.CodeChannelInvalid {
				t.Fatalf("expected CodeChannelInvalid, got %v", err)
			}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.run(t)
		})
	}
}

// --- Version ---

func TestParseDPVersion(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		t.Parallel()
		v, err := dp1.ParseDPVersion("1.1.0")
		if err != nil {
			t.Fatal(err)
		}
		if v.String() != "1.1.0" {
			t.Fatal(v.String())
		}
	})
	t.Run("invalid", func(t *testing.T) {
		t.Parallel()
		_, err := dp1.ParseDPVersion("not-semver")
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestWarnMajorMismatch(t *testing.T) {
	t.Run("mismatch", func(t *testing.T) {
		t.Parallel()
		v, _ := dp1.ParseDPVersion("2.0.0")
		if err := dp1.WarnMajorMismatch(v, 1); err == nil {
			t.Fatal("expected mismatch")
		}
		if err := dp1.WarnMajorMismatch(v, 2); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("nil_version", func(t *testing.T) {
		t.Parallel()
		if err := dp1.WarnMajorMismatch(nil, 1); err != nil {
			t.Fatal(err)
		}
	})
}

// --- Coded errors ---

func TestCodeFromValidationWrappers(t *testing.T) {
	cases := []struct {
		name string
		wrap func(error) error
		want dp1.ErrorCode
	}{
		{"playlist", dp1.CodeFromPlaylistValidation, dp1.CodePlaylistInvalid},
		{"playlist_group", dp1.CodeFromPlaylistGroupValidation, dp1.CodePlaylistGroupInvalid},
		{"ref_manifest", dp1.CodeFromRefManifestValidation, dp1.CodeRefManifestInvalid},
		{"channel", dp1.CodeFromChannelValidation, dp1.CodeChannelInvalid},
	}
	for _, tc := range cases {
		t.Run(tc.name+"_wraps_validation", func(t *testing.T) {
			t.Parallel()
			err := tc.wrap(dp1.ErrValidation)
			if err == nil {
				t.Fatal("expected error")
			}
			var coded *dp1.CodedError
			if !errors.As(err, &coded) || coded.Code != tc.want {
				t.Fatalf("got %v", err)
			}
		})
		t.Run(tc.name+"_passthrough_other", func(t *testing.T) {
			t.Parallel()
			e := errors.New("other")
			if got := tc.wrap(e); !errors.Is(got, e) {
				t.Fatal("expected same error")
			}
		})
	}
}

func TestCodedError_and_WithCode(t *testing.T) {
	t.Run("unwrap", func(t *testing.T) {
		t.Parallel()
		inner := errors.New("inner")
		err := dp1.WithCode(dp1.CodePlaylistInvalid, inner)
		var ce *dp1.CodedError
		if !errors.As(err, &ce) {
			t.Fatal("expected CodedError")
		}
		if !errors.Is(err, inner) {
			t.Fatal("unwrap")
		}
		if ce.Error() == "" {
			t.Fatal("empty")
		}
	})
	t.Run("nil", func(t *testing.T) {
		t.Parallel()
		if dp1.WithCode(dp1.CodePlaylistInvalid, nil) != nil {
			t.Fatal("expected nil")
		}
	})
}

// §3.6 puts the inline manifest's bytes inside the signed payload with no refHash of their own,
// so a decode/re-encode must not disturb them. Holding the field as raw JSON is what makes that
// true: the §3.6 example carries an artist with "id": "", which omitempty on a decoded manifest
// would drop, changing the JCS payload and invalidating a signature on an untouched document.
//
// The guarantee is scoped to the manifest subtree — omitempty elsewhere in these structs can
// still drop present-but-empty fields, so raw bytes remain the safe thing to sign in general.
func TestInlineManifest_survivesStructRoundTripForSigning(t *testing.T) {
	t.Parallel()
	_, priv, _ := ed25519.GenerateKey(nil)
	body := []byte(`{"dpVersion":"1.1.0","title":"Inline","items":[{"source":"https://example.com/a",` +
		`"inlineManifest":{"refVersion":"0.1.0","id":"ref-9d26ecb3","created":"2026-07-28T00:00:00Z","locale":"en",` +
		`"metadata":{"title":"Pre-Process","artists":[{"name":"Casey Reas","id":""}]}}}]}`)
	sig, err := sign.SignMultiEd25519(body, priv, playlist.RoleCurator, "2026-07-28T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	sigJSON, _ := json.Marshal([]playlist.Signature{sig})
	signed := append(body[:len(body)-1], []byte(`,"signatures":`+string(sigJSON)+`}`)...)

	if _, err := dp1.ParseAndValidatePlaylistWithPlaylistsExtension(signed); err != nil {
		t.Fatal(err)
	}
	ok, failed, err := sign.VerifyPlaylistSignatures(signed)
	if err != nil || !ok {
		t.Fatalf("raw bytes must verify: ok=%v failed=%+v err=%v", ok, failed, err)
	}

	// Same document, re-encoded from the decoded structs.
	decoded, err := dp1.ParseAndValidatePlaylistWithPlaylistsExtension(signed)
	if err != nil {
		t.Fatal(err)
	}
	remarshaled, err := json.Marshal(decoded)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(remarshaled), `"id":""`) {
		t.Fatalf("the empty artist id was dropped, so inlineManifest is no longer raw JSON: %s", remarshaled)
	}
	ok, failed, err = sign.VerifyPlaylistSignatures(remarshaled)
	if err != nil || !ok {
		t.Fatalf("round-tripped document must still verify: ok=%v failed=%+v err=%v", ok, failed, err)
	}
}

// JSON member names are case-sensitive and the item schemas permit properties they do not
// describe, so `{"contentRating":"mature","ContentRating":"general"}` validates as mature — the
// capitalized member is merely an unknown property. encoding/json's case-insensitive key fallback
// would otherwise match it to the field and hand the consumer "general", admitting mature work as
// general despite the signed rating. The decoded value must equal the validated one.
func TestContentRatingIgnoresCaseVariantMembers(t *testing.T) {
	t.Parallel()
	const sigBlock = `"signatures":[{"alg":"ed25519","kid":"did:key:z6MkhaXgBZDvotDkL5257faiztiGiC2QtKLGpbnnEGta2doK","ts":"2025-01-01T00:00:00Z","payload_hash":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","role":"curator","sig":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"}]`
	for _, tc := range []struct {
		name string
		item string
		want string // "" means the rating must be absent
	}{
		{"exact wins over capitalized", `"source":"https://a","contentRating":"mature","ContentRating":"general"`, "mature"},
		{"capitalized alone is not a rating", `"source":"https://a","ContentRating":"general"`, ""},
		{"upper alone is not a rating", `"source":"https://a","CONTENTRATING":"general"`, ""},
		{"lower alone is not a rating", `"source":"https://a","contentrating":"general"`, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			doc := []byte(`{"dpVersion":"1.1.0","title":"x","items":[{` + tc.item + `}],` + sigBlock + `}`)
			for name, parse := range map[string]func([]byte) (*playlist.Playlist, error){
				"core":                     dp1.ParseAndValidatePlaylist,
				"playlists":                dp1.ParseAndValidatePlaylistWithPlaylistsExtension,
				"content-rating":           dp1.ParseAndValidatePlaylistWithContentRatingExtension,
				"playlists+content-rating": dp1.ParseAndValidatePlaylistWithPlaylistsAndContentRatingExtensions,
			} {
				out, err := parse(doc)
				if err != nil {
					t.Fatalf("%s parser: %v", name, err)
				}
				got := out.Items[0].ContentRating
				if tc.want == "" {
					if got != nil {
						t.Fatalf("%s parser: want no rating, got %q", name, *got)
					}
					continue
				}
				if got == nil || string(*got) != tc.want {
					t.Fatalf("%s parser: want %q, got %v", name, tc.want, got)
				}
			}
		})
	}

	// contentReasons takes the same exact-key treatment.
	doc := []byte(`{"dpVersion":"1.1.0","title":"x","items":[{"source":"https://a","contentReasons":["nudity"],"ContentReasons":["language"]}],` + sigBlock + `}`)
	out, err := dp1.ParseAndValidatePlaylistWithPlaylistsAndContentRatingExtensions(doc)
	if err != nil {
		t.Fatal(err)
	}
	if out.Items[0].ContentReasons == nil || len(*out.Items[0].ContentReasons) != 1 ||
		(*out.Items[0].ContentReasons)[0] != "nudity" {
		t.Fatalf("reasons: %v", out.Items[0].ContentReasons)
	}
}
