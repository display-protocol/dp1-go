# Changelog

Notable changes to the dp1-go SDK. The module is `v0.x`, so minor versions may break source
compatibility; every break is listed here with the migration.

## Unreleased — v0.6.0

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

**`playlist.MousePrefs.{Click,Scroll,Drag,Hover}`: `bool` → `*bool`**

**`playlist.InteractionPrefs.Keyboard`: `[]string` → `*[]string`**

A layer must be able to *revoke* an interaction, not only grant it, and with plain `bool` an
explicit `false` was indistinguishable from absent — so `mouse.click: false` on an item could not
switch off what a manifest had enabled, and `"keyboard": []` was a no-op that `omitempty` also
erased on re-encoding. This matches `DisplayPrefs.Autoplay` and `Loop`, already `*bool` for the
same reason. Keeping the exported types and tracking presence privately was rejected: only the
JSON decoder could set such a flag, so a caller building a playlist in Go could never express an
explicit `false`.

```go
// before
mouse := &playlist.MousePrefs{Click: true}
kb := &playlist.InteractionPrefs{Keyboard: []string{"KeyA"}}
if mouse.Click { … }

// after
mouse := &playlist.MousePrefs{Click: playlist.Bool(true)}
kb := &playlist.InteractionPrefs{Keyboard: playlist.Keys("KeyA")}
if mouse.Click != nil && *mouse.Click { … }
```

`playlist.Keys()` with no arguments is an explicit empty list — a revocation — not absence.

### Fixed

- A manifest's `interaction` block merges by JSON field presence. A ref manifest setting only
  `mouse.scroll` no longer erases a `mouse.click` a lower layer set.

### Known gaps

Found while implementing the above, tracked separately so this release stays scoped to the two
spec changes:

- [#7](https://github.com/display-protocol/dp1-go/issues/7) — `merge.DisplayForItem` returns
  pointers into the playlist defaults, and thumbnail dimensions spelled `1e2` or `100.0` validate
  against the schema but fail to decode into `int`.
