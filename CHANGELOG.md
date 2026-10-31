# Changelog

Notable changes to the dp1-go SDK. The module is `v0.x`, so minor versions may break source
compatibility; every break is listed here with the migration.

## v0.7.2 — 2026-10-31

Catches the embedded schemas up with the rest of
[display-protocol/dp1#55](https://github.com/display-protocol/dp1/pull/55) and pins them to a spec
commit. The schemas already accepted the six members below, but the structs had no fields for
them: `json.Unmarshal` dropped them, and a strict decoder built on these types (dp1-feed-v2's
request models) refused the document.

### Added

- `playlist.Playlist.Visibility` and `playlists.Overlay.Visibility` (`*string`) — the playlists
  extension's listing request (§3.7): `public` or `unlisted`. Nil means absent, which is public. A
  pointer because the vocabulary is open and every present value other than exactly `public`,
  including `""`, means unlisted.
- `playlists.Note.Title` and `playlists.Note.Author` (an `identity.Entity`; nil means the note is
  the curators' own).
- `refmanifest.Metadata.Medium` (`Type` from a closed enumeration, `Details` as display lines),
  `refmanifest.Controls.Permissions` (`camera`, `microphone`) and
  `refmanifest.DisplayControls.ImageRendering` — refVersion 1.2.0.
- `scripts/check-schema-sync.sh` and the Schema sync workflow: `internal/schema/` must be byte
  copies of display-protocol/dp1 at the commit in `internal/schema/dp1-commit.txt`.
  `playlist-group.json` is the one embedded file the spec no longer has (removed in dp1#41); it
  stays while dp1-feed-v2, dp1-cli and dp1-validator import `playlistgroup`.

### Changed

- `merge.DisplayForItem` applies a manifest's `imageRendering` at the manifest's place in the
  resolution order: above defaults and the inline copy, below override and item-local fields.
- The channels extension schema is registered under its spec `$id`,
  `extensions/channels/v1.1.0/schema.json`; it was embedded under v1.0.0.
- Schema descriptions follow the spec text at the pinned commit.

### Compatibility

Documents that validated before still do, unless they use one of the six members with a value the
new schemas reject — a non-string `visibility`, a note `title` outside 1–200 characters or an
`author` without `name`, an empty, repeated or unknown `permissions` entry, a `medium` with an
unknown `type`, an extra member or an empty `details` line, an `imageRendering` other than `auto`
or `pixelated`. The old schemas did not describe these members, so any value passed.

`Playlist`, `Overlay`, `Note`, `Metadata`, `Controls` and `DisplayControls` gain exported fields,
so unkeyed composite literals of them stop compiling; keyed literals are unaffected.

### Known limits

- On the core-only path (`ParseAndValidatePlaylist`), a `visibility` or note `title`/`author` of
  the wrong JSON type fails the decode, as `openAt`, `summary`, `curators` and `note` already do;
  only `inlineManifest` and the content-rating members are tolerated there.
- `visibility` matches case-insensitively, like every field except the content-rating members.
  A directory deciding what to list should read the member by exact key from the signed bytes.
  An exact-key `Playlist.UnmarshalJSON` is ruled out: it would be promoted into structs that embed
  `Playlist` (ffos-user does) and take over their decoding.

## v0.7.1 — 2026-10-29

### Fixed

- `playlist.SeriesID` no longer lets a negative exponent reach `big.Rat` unbounded. The digit
  bound clamped a negative exponent to zero on the reasoning that it "only shrinks the value" —
  true of the value, false of the work: `big.Rat` holds `m e -k` as a fraction over `10^k`, so
  `SetString` materialized a k-digit denominator before `IsInt` could reject the literal as
  fractional. `{"seriesId": 1e-1000000}` is ten bytes on the wire and allocated roughly 3 MB,
  the same amplification the positive branch already refused, reached through the denominator
  instead of the numerator. The bound now reads the exponent's magnitude and ignores its sign,
  and bounds that magnitude on its own instead of against the mantissa — the denominator is
  the only amplification a negative exponent buys, while the mantissa's digits are already on
  the wire. A negative exponent the mantissa genuinely absorbs still decodes however long the
  mantissa is: `1500e-2` is `15`, and `1` followed by 512 zeros times `10^-512` is `1`. Every
  literal that decoded on v0.7.0 still decodes; the only refusals added are fractional ones
  `IsInt` already rejected, so what changes there is which error comes back, not whether one
  does. Reported against a feed built on v0.7.0.
- `playlist.SeriesID`'s digit bound is now total at the limits of `int`. It range-checked the
  exponent only after arithmetic on it, and Go's signed overflow wraps: `-math.MinInt64` is
  `math.MinInt64` again, and `math.MaxInt64 + len(mantissa)` wraps negative, so either limit
  satisfied the bound and passed the literal through. No amplification was reachable that way —
  `big.Rat.SetString` refuses an exponent that large on its own, for about 1.5 KB — so this was
  a guard correct only by accident of another package's internal limit. The range check now
  runs before the arithmetic. Closing that hole must not cost a zero mantissa its place:
  `0e9223372036854775807` is the integer `0`, `big.Rat` reads it as `0` for under 100 bytes
  without applying the exponent at all, and it decoded on v0.7.0 precisely because the sum
  wrapped. A mantissa with no non-zero digit is therefore accepted ahead of either magnitude
  test, on both signs — there is no expansion to bound.
- The doc comment on that bound claimed no path inside the module reaches it because the
  `ParseAndValidate*` entrypoints validate first. That is false for underflow: `internal/validate`
  decodes to `any` without `UseNumber`, and `encoding/json` errors on an overflowing literal
  (`1e400`) but turns an underflowing one into `float64 0` silently. Schema validation passes such
  a document and the decode that follows reaches the bound, so it guards the primary entrypoint,
  not merely a bare `json.Unmarshal`.

## v0.7.0 — 2026-10-29

Catches the SDK up with the DP-1 spec's Entity consolidation and
[display-protocol/dp1#55](https://github.com/display-protocol/dp1/pull/55), found by publishing a
real exhibition corpus through a feed built on this module. A minor bump, not a patch: one field
is retyped (see Changed).

### Added

- `playlist.Playlist.OpenAt` — when the playlist's own subject became public (an exhibition's
  opening), distinct from `Created`, which is the document's issuance time, and from item-level
  `DisplayAt` scheduling, which gates eligibility. `OpenAt` sets no eligibility, timer, or
  activation (dp1#55).
- `playlist.Playlist.Links` — editorial content about the playlist (articles, events, news), in
  display order. `identity.Link` gains `Title`, `Date`, `Thumbnail`, and `Authors` for that use,
  and its type enumeration gains `profile`, `publication`, `article`, `event`, and `news`.
- `identity.Entity` gains `Role`, `Location`, `Addresses`, `Avatar`, `Biographies`, and `Links`.
  Curators, publishers, and `metadata.artists[]` all name entities, and until now any of them
  carrying one of these was rejected as an unknown field. `Key` is no longer required — the
  spec's Entity requires only `name`, and a wallet-based publisher may hold no DID.
- `refmanifest.Artist` gains `Key`, `Role`, and `Location`; `refmanifest.Link` gains `Title`,
  `Date`, `Thumbnail`, and `Authors`. The ref-manifest schema's `Artist` is now a `$ref` to its
  `Entity`, so without these a manifest carrying them would validate and then lose them on
  decode. The Go struct keeps its own name and is not aliased to `identity.Entity`.
- `playlist.DisplayPrefs.ImageRendering` (`auto` | `pixelated`), resolved by `merge` in the usual
  precedence order. The ref-manifest `DisplayControls` has no counterpart, so a fetched manifest
  cannot set or clear it.

### Changed (breaking)

**`playlist.ProvenanceContract.SeriesID`: `*int` → `*playlist.SeriesID`**

The spec types `provenance.contract.seriesId` as `oneOf [integer, string]` — a decimal string for
ids too large for a JSON number to hold exactly, the same reason `tokenId` is a string. With
`*int`, half the wire format did not decode at all, and a 256-bit id could not be represented.

`SeriesID` is a string type that accepts either wire shape on decode and always re-emits a
string, which satisfies the `oneOf` either way and never risks the float64 precision loss a
round-tripped large integer would.

```go
// before
if c.SeriesID != nil { use(*c.SeriesID) }        // int

// after
if c.SeriesID != nil { use(string(*c.SeriesID)) } // decimal digits
// numeric comparison, when you need one:
n, err := strconv.ParseUint(string(*c.SeriesID), 10, 64)
// constructing
id := playlist.SeriesID("5")
c := playlist.ProvenanceContract{SeriesID: &id}
```

Decode rejects anything the schema's own `oneOf` rejects — a sign, a decimal point, an empty or
non-digit string — rather than accepting it and re-emitting a document this SDK would refuse to
validate. An exponent-form literal is accepted and canonicalized (`5e2` → `"500"`) up to 1024
expanded digits; past that it is refused, because the expansion is unbounded work an untrusted
document would otherwise buy for a handful of bytes.

## v0.6.2 — 2026-09-15

Adds the draft Content Rating Extension, spec text merged in
[display-protocol/dp1#52](https://github.com/display-protocol/dp1/pull/52). Additive: every document
that validated before still does. `PlaylistItem` gains two exported fields (see Source compatibility).

- Add draft content-rating extension types, embedded schemas, and parse helpers. `contentRating`
  is an optional per-item label; absence is unrated. Optional `contentReasons` are nonempty
  open-vocabulary strings.
- **The rating vocabulary is open** ([spec §3.3](https://github.com/display-protocol/dp1/pull/52)):
  any string is a valid rating, `v0.1.0` defines only `general` and `mature`, and a value the
  consumer does not recognize means unrated — nothing is assumed from an unknown label, so only
  `mature` hides anything. `contentrating.Rating` stays a string type with those two constants;
  use the new `Rating.Known()` to branch on the values this SDK version defines, never to decide
  whether a document is valid. `null` and non-string values remain schema-invalid.
- Dynamic-query items now use the combined playlists + content-rating validator, rejecting
  ratings of the wrong JSON type and malformed reasons while preserving unrated and
  unrecognized-rating items.
- Reserve `contentBlocked` for valid items excluded by consumer policy. Invalid metadata remains
  `playlistInvalid`. Existing JCS/signature behavior is unchanged; extension fields are signed.

### Source compatibility

`playlist.PlaylistItem` gains two exported fields (`ContentRating`, `ContentReasons`). No existing
field is removed or retyped, so keyed literals, field access, and JSON round-trips are unaffected.
An **unkeyed** composite literal — `playlist.PlaylistItem{"https://a", …}` — must supply every
field positionally and will fail to compile; migrate it to a keyed literal, which `go vet`'s
`composites` check already recommends for a struct from another package. This matches how v0.6.0
handled adding `InlineManifest` to the same struct (listed under Added, not under the breaking
section, which was reserved for the `Thumbnail.W`/`.H` retype).

### Parser tolerance

`playlist.PlaylistItem` now has an `UnmarshalJSON` that decodes the two content-rating members
leniently. Core DP-1 and the playlists extension both permit item properties they do not
describe, so `ParseAndValidatePlaylist` accepts a document carrying `"contentRating": 1`; with a
plain typed field the decode step that follows schema validation would then fail on a document
the schema had just accepted, locking a consumer that never opted into this draft extension out
of the playlist entirely. A member whose JSON type does not fit is left nil — the same "absent,
therefore unrated" state that consumer saw before the extension existed.

This is not leniency on the paths that implement the extension:
`ParseAndValidatePlaylistWithContentRatingExtension` and its combined sibling validate against
the extension schema *before* decoding, so a rating of the wrong JSON type is rejected there and
never reaches the tolerant decode.

Note this tolerance is only about JSON *type*, never about vocabulary. Every rating string —
including one this SDK version does not define — decodes and re-encodes byte-identically on every
parse path, so relaying a document with an unfamiliar rating leaves the JCS payload and the
signature over it intact. Only a value the spec already calls invalid (`null`, or a non-string)
is dropped on re-encode, and then only by the parsers that never validated the field. Sign and
verify the original bytes, per §7.1, not a re-encode.

Both members are taken by **exact JSON key**. Member names are case-sensitive and the item
schemas permit properties they do not describe, so `{"contentRating":"mature","ContentRating":
"general"}` validates as mature — the capitalized member is merely an unknown property.
`encoding/json`'s case-insensitive key fallback would otherwise match it to the field and report
`general`, admitting mature work as general despite the signed rating. Exact lookup keeps the
decoded value equal to the validated one. This is specific to the two content-rating members,
because they gate whether an item is shown; every other field still matches case-insensitively,
which is `encoding/json`'s behavior throughout this SDK.

Note that `note` and `displayAt` — the typed playlists-extension fields shipped through v0.6.1 —
still have the untolerated behavior on the core-only path; only `inlineManifest` (raw) and now
the content-rating members avoid it. Making those two consistent is a separate change.

## v0.6.1 — 2026-09-14

Aligns the SDK with the artist profile added to the Ref Manifest in
[display-protocol/dp1#51](https://github.com/display-protocol/dp1/pull/51) (refVersion 1.1.0,
core `ref-manifest.md` §4.1). Additive: every document that validated before still does, and no
exported type changes shape.

### Added

- `refmanifest.Artist` gains `Addresses []string`, `Avatar *Thumbnail`, `Biographies []Biography`
  and `Links []Link`, with new `refmanifest.Biography` (`Text`, `Source`, `SourceURL`) and
  `refmanifest.Link` (`Type`, `URL`) and the `LinkTypeWebsite` / `LinkTypeTwitter` /
  `LinkTypeInstagram` / `LinkTypeOther` constants for the schema's `links[].type` enumeration.
- The embedded `core/v1.1.0/ref-manifest.json` is byte-identical to upstream `main`, so
  `ParseAndValidateRefManifest` now enforces the new constraints: each address non-empty, each
  biography with non-empty `text`, each link with a full URL (`format: uri`, so a bare handle such
  as `@REAS` is rejected) and a `type` from the enumeration.

### Deprecated

- `refmanifest.Artist.URL` — the 1.0.0 single profile URL. Still accepted on the wire; write a
  `Links` entry of type `LinkTypeWebsite` instead. Per the spec, a producer that emits both keeps
  them equal and consumers read `Links` first, falling back to `URL` only when `Links` is absent
  or empty. The SDK checks neither rule; it validates shape only.

### Not enforced, by design

`Addresses` is the only field the spec lets a consumer use to recognise one artist across
producers, and it is a signed claim of the producer rather than a fact: a wallet can be shared, so
two records whose address lists intersect are not thereby one artist, and contract addresses
(Tezos `KT1…`, EVM collection contracts) must not be listed. None of that is expressible in JSON
Schema, so the SDK does not check it — the same posture it takes for every other prose rule.

## v0.6.0 — 2026-08-14

Aligns the SDK with two DP-1 spec changes: inline carriage of a Ref Manifest on a playlist item
(Playlist Extension §3.6) and the relaxation of the ref-manifest `Thumbnail` requirements.

### Added

- `playlist.PlaylistItem.InlineManifest` (`json.RawMessage`) — a complete Ref Manifest carried on
  an item instead of behind `ref` (§3.6). Decode it with `PlaylistItem.ParseInlineManifest()`, or
  pass the raw bytes to `dp1.ParseAndValidateRefManifest` for decode plus schema validation.

  Raw JSON rather than a decoded manifest for two reasons. The core schema describes no extension
  field and core DP-1 tolerates unknown ones, so a core-only player must be able to parse a
  playlist carrying an `inlineManifest` it does not implement — a typed field would fail the
  decode step on documents the core schema had just accepted. And the bytes are covered by the
  playlist signature with no `refHash` counterpart, so they must survive a decode/re-encode
  unchanged; re-encoding a decoded manifest drops present-but-empty fields (the artist `"id": ""`
  in the §3.6 example) and changes the JCS payload.
- `merge.ManifestForItem(ref, item)` — the §3.6 precedence for non-display fields: a fetched
  `ref` manifest wins, the inline copy is the offline fallback.

### Changed (breaking)

**`refmanifest.Thumbnail.W` / `.H`: `int` → `*int`**

The ref-manifest schema no longer requires `w`/`h` (core changelog 2026-08-12); producers holding
only a bare thumbnail URL omit them, and consumers must treat them as possibly absent. With `int`,
an absent dimension decoded to `0` — a value the schema forbids (`minimum: 1`) — and re-encoding
emitted `"w":0,"h":0`, producing an invalid document.

```go
// before
if th.W > 0 { use(th.W) }
// after
if th.W != nil { use(*th.W) }

// constructing
w, h := 1200, 900
th := refmanifest.Thumbnail{URI: "…", W: &w, H: &h}
```

### Known gaps

Found while implementing the above, tracked separately so this release stays scoped to the two
spec changes:

- [#6](https://github.com/display-protocol/dp1-go/issues/6) — interaction settings merge by Go
  zero value, so a higher-precedence layer can only switch an interaction on, never off, and a
  manifest's `mouse` block replaces the lower layer's wholesale. Predates inline manifests but is
  easier to reach now that two manifests stack.
- [#7](https://github.com/display-protocol/dp1-go/issues/7) — `merge.DisplayForItem` returns
  pointers into the playlist defaults, and thumbnail dimensions spelled `1e2` or `100.0` validate
  against the schema but fail to decode into `int`.
