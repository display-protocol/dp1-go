# Changelog

Notable changes to the dp1-go SDK. The module is `v0.x`, so minor versions may break source
compatibility; every break is listed here with the migration.

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
