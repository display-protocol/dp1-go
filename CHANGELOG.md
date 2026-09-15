# Changelog

Notable changes to the dp1-go SDK. The module is `v0.x`, so minor versions may break source
compatibility; every break is listed here with the migration.

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
