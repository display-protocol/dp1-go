# Changelog

Notable changes to the dp1-go SDK. The module is `v0.x`, so minor versions may break source
compatibility; every break is listed here with the migration.

## Unreleased — v0.6.0

Aligns the SDK with two DP-1 spec changes. Three exported types change shape, all for the same
reason: a field that can be **absent** must not be represented by a Go zero value that also means
something. Each break is source-breaking but mechanical, and the compiler finds every site.

### Added

- `playlist.PlaylistItem.InlineManifest` (`json.RawMessage`) — a complete Ref Manifest carried on
  an item instead of behind `ref` (Playlist Extension §3.6). Decode it with
  `PlaylistItem.ParseInlineManifest()`, or pass the raw bytes to `dp1.ParseAndValidateRefManifest`
  for decode plus schema validation.
- `merge.ManifestForItem(ref, item)` — the §3.6 precedence for non-display fields: a fetched
  `ref` manifest wins, the inline copy is the offline fallback.
- `playlist.Bool` and `playlist.Keys` — constructors for the optional fields below.

### Changed (breaking)

**1. `refmanifest.Thumbnail.W` / `.H`: `int` → `*int`**

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

**2. `playlist.MousePrefs.{Click,Scroll,Drag,Hover}`: `bool` → `*bool`**

**3. `playlist.InteractionPrefs.Keyboard`: `[]string` → `*[]string`**

Inline manifests mean two manifests now stack (`defaults → inlineManifest → ref → override →
item-local`), so a layer must be able to *revoke* an interaction, not only grant it. With plain
`bool`, `false` was indistinguishable from absent, so `mouse.click: false` on an item could not
switch off what a manifest had enabled — the merge order said the item wins, and it did not. The
same applied to `"keyboard": []`, whose `omitempty` also erased the revocation on re-encoding.

This matches `DisplayPrefs.Autoplay` and `Loop`, which were already `*bool` for this reason.

```go
// before
mouse := &playlist.MousePrefs{Click: true}
kb := &playlist.InteractionPrefs{Keyboard: []string{"KeyA"}}
if mouse.Click { … }
for _, k := range kb.Keyboard { … }

// after
mouse := &playlist.MousePrefs{Click: playlist.Bool(true)}
kb := &playlist.InteractionPrefs{Keyboard: playlist.Keys("KeyA")}
if mouse.Click != nil && *mouse.Click { … }
if kb.Keyboard != nil { for _, k := range *kb.Keyboard { … } }
```

`playlist.Keys()` with no arguments is an explicit empty list — a revocation — which is not the
same as leaving the field nil.

**Why not keep the exported types and track presence privately?** A private flag can only be set
by the JSON decoder, so a caller building a playlist in Go could never express an explicit
`false` — revocation would work for parsed documents and silently not work for constructed ones.
It would also not remove the API change: `merge` lives in another package and would need exported
accessors to read that state.

### Fixed

- `merge.DisplayForItem` merges a manifest's `interaction` block by JSON field presence. A ref
  manifest setting only `mouse.scroll` no longer erases a `mouse.click` an inline manifest set.
- `merge.DisplayForItem` no longer returns display preferences that share pointers with the
  playlist defaults. Writing through the result used to corrupt the baseline for every later item.
