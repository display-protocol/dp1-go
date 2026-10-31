// Package playlist defines DP-1 major-1 playlist document types (core wire format).
package playlist

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/display-protocol/dp1-go/extension/contentrating"
	"github.com/display-protocol/dp1-go/extension/identity"
	"github.com/display-protocol/dp1-go/extension/playlists"
	"github.com/display-protocol/dp1-go/refmanifest"
)

// Playlist is a DP-1 playlist (v1.0.x legacy signature and/or v1.1+ multi-signature).
type Playlist struct {
	DPVersion  string         `json:"dpVersion"`
	ID         string         `json:"id,omitempty"`
	Title      string         `json:"title"`
	Slug       string         `json:"slug,omitempty"`
	Created    string         `json:"created,omitempty"`
	Defaults   *Defaults      `json:"defaults,omitempty"`
	Items      []PlaylistItem `json:"items"`
	Signatures []Signature    `json:"signatures,omitempty"` // v1.1+
	Signature  string         `json:"signature,omitempty"`  // legacy v1.0.x (deprecated)

	// --- Registry extension: "playlists" (draft) — additive optional fields only ---
	// These are not part of DP-1 core JSON Schema; they are validated only when using
	// ParseAndValidatePlaylistWithPlaylistsExtension. Safe to omit for core-only documents.
	Note     *playlists.Note   `json:"note,omitempty"`
	Curators []identity.Entity `json:"curators,omitempty"`
	Summary  string            `json:"summary,omitempty"`
	// OpenAt is when this playlist's own subject became public — an exhibition's
	// opening, a collection's release. Reuses the DisplayAt wire shape for one consistent
	// date format across DP-1, but is NOT displayAt Scheduling (§3.5 on PlaylistItem):
	// that gates which items are eligible to play; OpenAt is a playlist-level fact for
	// ordering and display only, and sets no eligibility, timer, or activation. Distinct
	// from Created too, which is the document's own issuance time and churns on every
	// re-export; OpenAt is the real-world event and does not (dp1#55).
	OpenAt       string                  `json:"openAt,omitempty"`
	CoverImage   string                  `json:"coverImage,omitempty"`
	DynamicQuery *playlists.DynamicQuery `json:"dynamicQuery,omitempty"`
	// Links is editorial content about the playlist (articles, events, news), in the
	// order to show them — references only, the content itself stays at the URL (dp1#55).
	Links []identity.Link `json:"links,omitempty"`
	// Visibility asks directories to leave the playlist out of their listings (playlists
	// extension §3.7): "public" or "unlisted". A listing request, not access control; it has no
	// effect on playback. Nil means the member is absent, which is public. A pointer because the
	// vocabulary is open and every present value other than exactly "public" — including "" —
	// means unlisted: a plain string would read "" as absent and drop it on re-encode.
	//
	// A directory deciding what to list should read the member by exact key from the signed
	// bytes, as dp1-feed-v2 does. encoding/json matches keys case-insensitively, so a document
	// carrying both "visibility" and an unknown "Visibility" member can decode to the second.
	// PlaylistItem takes its content-rating members by exact key for this reason; Playlist does
	// not, because a Playlist.UnmarshalJSON would be promoted into structs that embed Playlist
	// (ffos-user's dp1.Playlist does) and take over their decoding, dropping their own fields.
	Visibility *string `json:"visibility,omitempty"`
}

// Defaults holds playlist-level defaults inherited by items.
type Defaults struct {
	Display  *DisplayPrefs `json:"display,omitempty"`
	License  string        `json:"license,omitempty"`
	Duration *float64      `json:"duration,omitempty"`
}

// PlaylistItem is one entry in a playlist.
type PlaylistItem struct {
	ID         string           `json:"id,omitempty"`
	Slug       string           `json:"slug,omitempty"`
	Title      string           `json:"title,omitempty"`
	Source     string           `json:"source"`
	Duration   *float64         `json:"duration,omitempty"`
	License    string           `json:"license,omitempty"`
	Ref        string           `json:"ref,omitempty"`
	RefHash    string           `json:"refHash,omitempty"` // prose spec; not in core JSON Schema yet
	Override   json.RawMessage  `json:"override,omitempty"`
	Display    *DisplayPrefs    `json:"display,omitempty"`
	Repro      *ReproBlock      `json:"repro,omitempty"`
	Provenance *ProvenanceBlock `json:"provenance,omitempty"`

	// Playlists extension only (ParseAndValidatePlaylistWithPlaylistsExtension); not in core schema.
	Note *playlists.Note `json:"note,omitempty"`
	// DisplayAt is an optional ISO 8601 scheduling datetime (§3.5.2; date-only not accepted).
	// Nil means the field is absent (evergreen when displayAt scheduling is active). A non-nil pointer
	// (including to "") means the field is present; unresolvable values are not eligible (§3.5.5).
	DisplayAt *string `json:"displayAt,omitempty"`
	// InlineManifest carries a complete Ref Manifest inline instead of behind Ref (§3.6):
	// the same document, checked by the unmodified ref-manifest schema, so a malformed one
	// invalidates the playlist on the playlists-extension path.
	//
	// Held as raw JSON, like Override, rather than as a decoded manifest. The core schema does
	// not describe this field and core DP-1 tolerates unknown ones, so a core-only player must
	// be able to parse a playlist carrying an inlineManifest it does not implement — including
	// one whose shape it would reject. A typed field would fail the decode step of
	// ParseAndValidatePlaylist on documents the core schema accepted. Keeping the bytes also
	// keeps them verbatim: they are covered by the playlist signature (core §7.1) with no
	// refHash counterpart, and re-encoding a decoded manifest would drop present-but-empty
	// fields (the artist "id": "" in the §3.6 example) and change the JCS payload.
	//
	// Use ParseInlineManifest to decode, or hand these bytes straight to
	// dp1.ParseAndValidateRefManifest for decode plus full schema validation.
	//
	// Precedence when both are present: defaults → inlineManifest → ref → item-local, i.e. a
	// fetched Ref manifest wins and the inline copy is the offline/degraded fallback.
	InlineManifest json.RawMessage `json:"inlineManifest,omitempty"`

	// ContentRating and ContentReasons are draft content-rating extension fields.
	//
	// Nil ContentRating means the field was absent, i.e. unrated. So, for a consumer, does a
	// rating this SDK version does not define: the wire vocabulary is open (extension §3.3), any
	// string is a valid rating, and nothing is assumed from a label the consumer does not
	// recognize — only "mature" hides anything. Check contentrating.Rating.Known before acting on
	// a value. Present null and non-string values are invalid and are rejected by the
	// content-rating-aware parsers.
	//
	// Parse with a content-rating-aware helper before acting on either field.
	//
	// Both are typed rather than raw, so UnmarshalJSON decodes them leniently — see the comment
	// there for why a typed extension field would otherwise break the core-only parser.
	ContentRating *contentrating.Rating `json:"contentRating,omitempty"`
	// Pointer-to-slice preserves absent versus a present empty array across decode/marshal.
	ContentReasons *[]string `json:"contentReasons,omitempty"`
}

// UnmarshalJSON decodes an item, tolerating content-rating members whose JSON type does not fit
// the typed fields.
//
// Core DP-1 and the playlists extension both permit item properties they do not describe, so
// ParseAndValidatePlaylist accepts a document carrying `"contentRating": 1`. With a plain typed
// field, the decode step that follows schema validation would then fail on a document the schema
// had just accepted, and a consumer that never opted into this draft extension could no longer
// read the playlist at all. That is the same schema/decode mismatch InlineManifest avoids by
// staying raw; these fields stay typed for the consumers that do implement the extension, and
// pay for it here instead.
//
// A member that does not fit is left nil — the same "absent, therefore unrated" state a
// core-only consumer would have seen before this extension existed. This is not a silent failure
// on the path that matters: ParseAndValidatePlaylistWithContentRatingExtension and its combined
// sibling validate against the extension schema *before* decoding, so a rating of the wrong JSON
// type is rejected loudly there and can never reach this leniency. Only the parsers that never
// checked the field in the first place see it.
//
// This tolerance is about JSON type only, never about vocabulary. Rating is a string type with
// no enumeration, so every rating string decodes here — including one from a later vocabulary —
// and re-encodes unchanged.
//
// Note the round-trip consequence, which therefore reaches only values the spec already calls
// invalid: re-encoding an item whose rating was null or a non-string drops that member and
// changes the JCS payload. Sign and verify the original bytes, as DP-1 §7.1 requires, not a
// re-encode.
func (it *PlaylistItem) UnmarshalJSON(data []byte) error {
	// Local type to strip this method, or json.Unmarshal would call it again. The outer raw
	// fields shadow the embedded typed ones: encoding/json prefers the shallower field.
	type item PlaylistItem

	// Seed the shadow from the receiver rather than starting zero. encoding/json merges into a
	// struct — it writes only the members the document carries and leaves the rest alone — and
	// callers rely on that: decoding into a reused variable, and decoding a playlist whose Items
	// slice already has elements, since json reuses the existing backing array. Starting from a
	// zero value would silently clear Title, DisplayAt, InlineManifest and the rest on a partial
	// document, and would blank the item outright on a literal null, which json treats as a no-op.
	// The two raw fields are also a sink: encoding/json falls back to a case-insensitive key
	// match, so without them a member spelled "ContentRating" would land in the embedded typed
	// field. Their values are deliberately not used — see the exact-key lookup below.
	shadow := struct {
		item
		ContentRating  json.RawMessage `json:"contentRating"`
		ContentReasons json.RawMessage `json:"contentReasons"`
	}{item: item(*it)}
	if err := json.Unmarshal(data, &shadow); err != nil {
		return err
	}
	*it = PlaylistItem(shadow.item)

	// Take both members by exact JSON key rather than from the shadow fields.
	//
	// JSON member names are case-sensitive and the item schemas permit properties they do not
	// describe, so `{"contentRating":"mature","ContentRating":"general"}` validates as mature —
	// the second member is merely an unknown property. encoding/json's case-insensitive fallback
	// would nonetheless match the second key to the field and materialize "general", handing a
	// content-policy consumer a rating the signed document does not carry and admitting mature
	// work as general. Exact lookup keeps the decoded value equal to the validated one, which is
	// the SDK's validate-before-decode contract, and is worth the extra pass over the item for
	// the one field that gates whether something is shown.
	//
	// A key that is absent leaves the seeded value alone; a key that is present replaces it,
	// including a present null or an untypeable value, which clear it.
	var members map[string]json.RawMessage
	if err := json.Unmarshal(data, &members); err != nil {
		// Not an object (a literal null reaches here as a nil map, not an error). The shadow
		// decode above already rejected anything that is not a valid item, so there is nothing
		// further to take.
		return nil //nolint:nilerr // shadow decode is authoritative for document-shape errors
	}

	if raw, ok := members["contentRating"]; ok {
		it.ContentRating = nil
		if string(raw) != "null" {
			var rating contentrating.Rating
			if json.Unmarshal(raw, &rating) == nil {
				it.ContentRating = &rating
			}
		}
	}
	if raw, ok := members["contentReasons"]; ok {
		it.ContentReasons = nil
		if string(raw) != "null" {
			var reasons []string
			if json.Unmarshal(raw, &reasons) == nil {
				if reasons == nil {
					// Defensive: only "null" decodes to nil, and it is excluded above. Keep the
					// present-versus-present-empty distinction the pointer-to-slice exists for.
					reasons = []string{}
				}
				it.ContentReasons = &reasons
			}
		}
	}
	return nil
}

// ParseInlineManifest decodes the item's inline Ref Manifest (§3.6).
//
// Returns (nil, nil) when the field is absent or JSON null, so callers can branch on the
// manifest alone. Decode only — the bytes are schema-checked when the playlist was parsed with
// dp1.ParseAndValidatePlaylistWithPlaylistsExtension; on the core-only path they are whatever
// the document carried, and an error here is how that surfaces. For decode plus validation,
// pass item.InlineManifest to dp1.ParseAndValidateRefManifest instead.
func (it PlaylistItem) ParseInlineManifest() (*refmanifest.Manifest, error) {
	if len(it.InlineManifest) == 0 || string(it.InlineManifest) == "null" {
		return nil, nil
	}
	var m refmanifest.Manifest
	if err := json.Unmarshal(it.InlineManifest, &m); err != nil {
		return nil, fmt.Errorf("playlist: decode inlineManifest: %w", err)
	}
	return &m, nil
}

// DisplayPrefs controls how a player renders an item (see DP-1 §4).
type DisplayPrefs struct {
	Scaling    string          `json:"scaling,omitempty"`
	Margin     json.RawMessage `json:"margin,omitempty"` // number or "%|vw|vh" string
	Background string          `json:"background,omitempty"`
	Autoplay   *bool           `json:"autoplay,omitempty"`
	Loop       *bool           `json:"loop,omitempty"`
	// ImageRendering is "auto" (let the player smooth) or "pixelated"
	// (nearest-neighbor, hard pixel edges — pixel art and bitmaps).
	// Independent of Scaling, which decides how the artwork fits.
	ImageRendering string            `json:"imageRendering,omitempty"`
	Interaction    *InteractionPrefs `json:"interaction,omitempty"`
	UserOverrides  map[string]bool   `json:"userOverrides,omitempty"`
}

// InteractionPrefs configures input (keyboard / mouse).
type InteractionPrefs struct {
	Keyboard []string    `json:"keyboard,omitempty"`
	Mouse    *MousePrefs `json:"mouse,omitempty"`
}

// MousePrefs toggles pointer interactions.
//
// These are plain bools, so an explicit false is indistinguishable from an absent key and the
// merge order cannot express a revocation: see #6, which changes them to pointers. Left as they
// are here to keep this change to the two spec updates it implements.
type MousePrefs struct {
	Click  bool `json:"click,omitempty"`
	Scroll bool `json:"scroll,omitempty"`
	Drag   bool `json:"drag,omitempty"`
	Hover  bool `json:"hover,omitempty"`
}

// ReproBlock carries deterministic reproduction hints (DP-1 §5).
type ReproBlock struct {
	EngineVersion map[string]string `json:"engineVersion,omitempty"`
	Seed          string            `json:"seed,omitempty"`
	AssetsSHA256  []string          `json:"assetsSHA256,omitempty"`
	FrameHash     *FrameHash        `json:"frameHash,omitempty"`
}

// FrameHash holds first-frame verification hashes.
type FrameHash struct {
	SHA256 string `json:"sha256,omitempty"`
	Phash  string `json:"phash,omitempty"`
}

// ProvenanceType identifies how provenance is expressed.
type ProvenanceType string

const (
	ProvenanceOnChain        ProvenanceType = "onChain"
	ProvenanceSeriesRegistry ProvenanceType = "seriesRegistry"
	ProvenanceOffChainURI    ProvenanceType = "offChainURI"
)

// ProvenanceBlock links rendered assets to chain or off-chain records (DP-1 §6).
type ProvenanceBlock struct {
	Type         ProvenanceType      `json:"type"`
	Contract     *ProvenanceContract `json:"contract,omitempty"`
	Dependencies []ProvenanceDep     `json:"dependencies,omitempty"`
}

// ProvenanceContract is contract-specific provenance data.
type ProvenanceContract struct {
	Chain    string    `json:"chain,omitempty"`
	Standard string    `json:"standard,omitempty"`
	Address  string    `json:"address,omitempty"`
	SeriesID *SeriesID `json:"seriesId,omitempty"`
	TokenID  string    `json:"tokenId,omitempty"`
	URI      string    `json:"uri,omitempty"`
	MetaHash string    `json:"metaHash,omitempty"`
}

// SeriesID is a provenance series identifier: an integer when the contract numbers its
// series with small values, or a decimal string when the identifier is too large for a
// JSON number to hold exactly (the same reason TokenID is a plain string) — the schema's
// seriesId is `oneOf [integer, string]`. Unmarshal accepts either wire shape; Marshal
// always re-emits a string, which satisfies that oneOf either way and never risks the
// float64 precision loss a round-tripped large integer would.
type SeriesID string

func (s *SeriesID) UnmarshalJSON(b []byte) error {
	trimmed := bytes.TrimSpace(b)
	if string(trimmed) == "null" {
		return nil
	}
	if len(trimmed) > 0 && trimmed[0] == '"' {
		var str string
		if err := json.Unmarshal(b, &str); err != nil {
			return fmt.Errorf("seriesId: %w", err)
		}
		if !seriesIDDigits(str) {
			return fmt.Errorf("seriesId: string form must be decimal digits, got %q", str)
		}
		*s = SeriesID(str)
		return nil
	}
	var num json.Number
	if err := json.Unmarshal(b, &num); err != nil {
		return fmt.Errorf("seriesId: must be an integer or a decimal string: %w", err)
	}
	lit := num.String()
	// Fast path: a bare run of digits is already canonical, because JSON forbids a
	// leading zero in a number. Taking it keeps every ordinary id away from big.Rat, so
	// the bound below only ever constrains spellings no producer emits.
	if seriesIDDigits(lit) {
		*s = SeriesID(lit)
		return nil
	}
	// Anything else — an exponent, a decimal point, a sign — needs big.Rat to be read
	// exactly. json.Number keeps the wire's raw spelling, so 5e2 and 1E+3 would otherwise
	// survive as-is and Marshal would re-emit them as strings that fail the schema's own
	// ^[0-9]+$ branch. big.Rat parses every JSON number spelling exactly, and
	// IsInt + Num().String() both rejects a fractional value and yields plain digits —
	// so the round trip is canonical whatever the producer wrote. big.Int alone would
	// reject 5e2, and strconv.ParseFloat would lose precision on the very large ids
	// SeriesID exists to carry.
	if !seriesIDWithinDigitBound(lit) {
		return fmt.Errorf("seriesId: integer too large, over %d digits: %s", maxSeriesIDDigits, lit)
	}
	rat, ok := new(big.Rat).SetString(lit)
	if !ok || !rat.IsInt() {
		return fmt.Errorf("seriesId: must be an integer or a decimal string, got %s", num.String())
	}
	canonical := rat.Num().String()
	// IsInt admits negatives, which the schema's integer branch excludes via
	// `minimum: 0`; the digit check below is what rejects them.
	if !seriesIDDigits(canonical) {
		return fmt.Errorf("seriesId: must be a non-negative integer, got %s", lit)
	}
	*s = SeriesID(canonical)
	return nil
}

// seriesIDDigits reports whether v is the schema's own spelling of a series id:
// `^[0-9]+$` — one or more digits, no sign, no point, not empty.
//
// Both wire branches funnel through it so a SeriesID decoded directly (json.Unmarshal
// on a Playlist, which README documents as a supported path) can never hold a value
// the validating ParseAndValidate* entrypoints would have refused. Without it the two
// paths disagreed: "-5", "5.5", "" and the JSON number -5 all decoded cleanly here and
// then re-marshaled as strings that fail the schema's oneOf on the way back out —
// a document this SDK produced that this SDK rejects.
func seriesIDDigits(v string) bool {
	if v == "" {
		return false
	}
	for i := 0; i < len(v); i++ {
		if v[i] < '0' || v[i] > '9' {
			return false
		}
	}
	return true
}

// maxSeriesIDDigits caps how many decimal digits an exponent-form seriesId may expand to.
// 2^256-1 — the largest id the string form exists to carry at all — is 78 digits, so this
// is more than an order of magnitude of headroom over anything a chain can mint.
const maxSeriesIDDigits = 1024

// seriesIDWithinDigitBound reports whether a JSON number literal expands to at most
// maxSeriesIDDigits decimal digits, deciding it from the exponent alone — before big.Rat
// is allowed to do the expansion.
//
// big.Rat.SetString's cost tracks the expanded digit count, not the literal's length, and
// it expands before IsInt or any digit check can refuse the value. That makes a bound on
// the literal's own length useless here: "1e1000000" is nine bytes and a megabyte of
// digits, roughly 100000x amplification, and a document of a few tens of KB can hold
// hundreds of them (the playlist schema puts no maxItems on items).
//
// The validated entrypoints do not make this bound redundant, and the comment that once
// said they did was wrong in the direction that matters. internal/validate decodes to
// `any` without UseNumber, and encoding/json is asymmetric there: an OVERFLOWING literal
// (1e400) errors out, but an UNDERFLOWING one (1e-1000000) becomes float64 0 with no
// error at all. Schema validation therefore passes it, and ParseAndValidate* then
// unmarshals the original bytes into the Go types — so this bound is reached on the
// primary documented path, not only on the bare-json.Unmarshal path that Playlist and
// PlaylistItem also support as exported structs. Do not delete this on the grounds that
// a validator runs first.
//
// Only exponent-form and decimal-point spellings arrive here — a literal written out in
// full took the digits fast path above and is never bounded, because it costs only as
// much work as it costs bytes and because refusing it would make this decoder reject a
// document the schema accepts (neither branch of the schema's oneOf carries a maxLength).
// The bound is on amplification, not on magnitude — which is why it reads the exponent's
// magnitude and not its sign: a negative exponent amplifies big.Rat's denominator exactly
// as a positive one amplifies its numerator. The two signs are bounded separately rather
// than summed, because the mantissa means something different to each; see below.
func seriesIDWithinDigitBound(lit string) bool {
	mantissa, exponent := lit, 0
	if i := strings.IndexAny(lit, "eE"); i >= 0 {
		mantissa = lit[:i]
		n, err := strconv.Atoi(lit[i+1:])
		if err != nil {
			// An exponent that does not even fit an int is far past the bound, and a
			// malformed one is big.Rat's to reject — either way, not worth expanding.
			return false
		}
		exponent = n
	}
	// A mantissa with no non-zero digit is the integer 0 whatever its exponent says, and
	// it is the one shape that absorbs an exponent of ANY size — so it is answered before
	// either magnitude test, and for both signs. Accepting it is safe only because
	// big.Rat declines to do the work: SetString returns at its own zero check before any
	// exponent is applied, on either sign, so `0e-1000000` and `0e9223372036854775807`
	// each cost about 50-100 bytes, as does a mantissa of a million zeros. There is no
	// expansion to bound, and these are integers the schema's integer branch accepts —
	// `0e-5000` and `0e9223372036854775807` both decoded to "0" on v0.7.0, the latter
	// only because the overflowing sum below happened to wrap into range.
	// TestSeriesID_ZeroMantissaCostsNothing locks the short-circuit this relies on; if a
	// future math/big built the 10^k denominator before looking at the mantissa, this
	// would become the very amplification vector the rest of the function refuses.
	//
	// It is deliberately not gated on len(mantissa): a zero mantissa costs what its bytes
	// cost however long it is, and gating it would re-refuse literals the base branch
	// accepted in the overflow band (1025 zeros times 10^math.MaxInt decoded to "0").
	if seriesIDMantissaIsZero(mantissa) {
		return true
	}
	// Every comparison below runs BEFORE any arithmetic on the exponent, because signed
	// overflow in Go wraps rather than traps and the exponent came off the wire: a
	// literal may name math.MaxInt or math.MinInt (`1e9223372036854775807` and
	// `1e-9223372036854775808` are ~23 bytes each). math.MaxInt + len(mantissa) wraps
	// negative and would satisfy `<= maxSeriesIDDigits`, admitting the literal; negating
	// math.MinInt yields math.MinInt again and used to do the same. Testing the exponent
	// first, and never negating it, is what makes the guard total.
	if exponent > maxSeriesIDDigits {
		return false
	}
	if exponent < 0 {
		// A negative exponent shrinks the VALUE but not the work: big.Rat holds
		// `m e -k` as a fraction over 10^k, so SetString materializes a k-digit
		// denominator in full — before norm() can cancel it against the mantissa, and
		// before IsInt can call the value fractional. 1e-1000000 is ten bytes and
		// roughly 3 MB of allocation, the same amplification the positive branch exists
		// to refuse, reached through the denominator instead of the numerator.
		//
		// The mantissa is deliberately NOT charged against that denominator budget, and
		// that is the one asymmetry between the signs. For a positive exponent the
		// expanded numerator is len(mantissa)+exponent digits, so summing them measures
		// the work. For a negative one the mantissa's digits are not amplification at
		// all — they are already on the wire, byte for byte — and the quotient is never
		// longer than the mantissa, so summing them measures nothing and refuses
		// ordinary integers: "1" followed by 512 zeros, times 10^-512, is the integer 1
		// and 518 bytes on the wire, and the sum called it 1025 digits and rejected it.
		// The two are therefore bounded independently, which also means this branch
		// refuses nothing the bound accepted before the sign was counted at all.
		if len(mantissa) > maxSeriesIDDigits {
			return false
		}
		// What is left is a non-zero mantissa (the zero case returned above) under an
		// exponent past the budget, and every such literal is fractional: the mantissa's
		// value is under 10^len(mantissa) <= 10^maxSeriesIDDigits < 10^|exponent|, so
		// 10^|exponent| cannot divide it and IsInt would refuse it anyway. Refusing here
		// only changes which error comes back — never whether one does — and saves the
		// expansion that producing the other error would cost.
		return exponent >= -maxSeriesIDDigits
	}
	return len(mantissa)+exponent <= maxSeriesIDDigits
}

// seriesIDMantissaIsZero reports whether a JSON number's mantissa carries no non-zero
// digit — "0", "000", "0.0" and "-0" all qualify. Such a literal is the integer 0 for any
// exponent of either sign, so no expansion can follow it; see the head of
// seriesIDWithinDigitBound for why that is the one shape exempt from the magnitude bound.
//
// It scans for a digit rather than comparing against "0" because the wire spelling is the
// producer's: json.Number hands over whatever was written, sign, point and all.
func seriesIDMantissaIsZero(mantissa string) bool {
	for i := 0; i < len(mantissa); i++ {
		if mantissa[i] >= '1' && mantissa[i] <= '9' {
			return false
		}
	}
	return true
}

func (s SeriesID) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(s))
}

// ProvenanceDep is documentary only; players must not fetch at runtime.
type ProvenanceDep struct {
	Chain    string `json:"chain,omitempty"`
	Standard string `json:"standard,omitempty"`
	URI      string `json:"uri,omitempty"`
}

// Signature is one entry in a v1.1+ signature chain (DP-1 §7.1.1).
type Signature struct {
	Alg         string `json:"alg"`
	Kid         string `json:"kid"`
	Ts          string `json:"ts"`
	PayloadHash string `json:"payload_hash"`
	Role        string `json:"role"`
	Sig         string `json:"sig"`
}

// Signature roles (DP-1 §7.1.1).
const (
	RoleCurator     = "curator"
	RoleFeed        = "feed"
	RoleAgent       = "agent"
	RoleInstitution = "institution"
	RoleLicensor    = "licensor"
)

// Algorithm values from the core schema (more may appear in future spec minors).
const (
	AlgEd25519        = "ed25519"
	AlgEIP191         = "eip191"
	AlgECDSASecp256k1 = "ecdsa-secp256k1"
	AlgECDSAP256      = "ecdsa-p256"
)
