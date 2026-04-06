# dp1-go

[![Test](https://github.com/display-protocol/dp1-go/actions/workflows/test.yaml/badge.svg)](https://github.com/display-protocol/dp1-go/actions/workflows/test.yaml?query=branch%3Amain)
[![Lint](https://github.com/display-protocol/dp1-go/actions/workflows/lint.yaml/badge.svg)](https://github.com/display-protocol/dp1-go/actions/workflows/lint.yaml?query=branch%3Amain)
[![codecov](https://codecov.io/gh/display-protocol/dp1-go/graph/badge.svg)](https://codecov.io/gh/display-protocol/dp1-go)

Go SDK for the [DP-1 protocol](https://github.com/display-protocol/dp1): playlists, playlist-groups (exhibitions), ref manifests, JCS signing payloads (RFC 8785), Ed25519 verification (legacy + multi-signature), and registered JSON Schema extensions.

**Module:** `github.com/display-protocol/dp1-go`

## Requirements

- Go 1.22+

## Install

```bash
go get github.com/display-protocol/dp1-go
```

## Usage

### Parse and validate

All entrypoints validate raw JSON against embedded JSON Schema (draft 2020-12), then decode into typed structs.

| Function | Document |
|----------|----------|
| `ParseAndValidatePlaylist` | Core playlist |
| `ParseAndValidatePlaylistWithPlaylistsExtension` | Core playlist + **playlists** extension overlay |
| `ParseAndValidatePlaylistGroup` | Playlist-group (exhibition) |
| `ParseAndValidateRefManifest` | Ref manifest |
| `ParseAndValidateChannel` | **channels** extension document |

```go
import "github.com/display-protocol/dp1-go"

p, err := dp1.ParseAndValidatePlaylist(playlistJSON)
if err != nil {
    return err
}

g, err := dp1.ParseAndValidatePlaylistGroup(groupJSON)
m, err := dp1.ParseAndValidateRefManifest(manifestJSON)
ch, err := dp1.ParseAndValidateChannel(channelJSON)
```

Playlist with the optional **playlists** extension overlay:

```go
p, err := dp1.ParseAndValidatePlaylistWithPlaylistsExtension(data)
```

Dynamic playlist items (playlists extension `dynamicQuery`): hydrate `{{placeholders}}` with `playlist.HydrationParams`, fetch the indexer, map response rows, validate each item against core `PlaylistItem`, and append after static items via `(*playlist.Playlist).ResolveDynamicQuery` (pass `*http.Client`, or `nil` for `http.DefaultClient`).

### Errors

- `errors.Is(err, dp1.ErrValidation)` — JSON Schema validation failed (after mapping, playlist failures still wrap `ErrValidation`).
- `errors.As` into `*dp1.CodedError` — stable `ErrorCode` for UI/telemetry (e.g. `dp1.CodePlaylistInvalid`; `dp1.CodeSigInvalid` is used by the `sign` package). Validation failures use codes such as `CodePlaylistInvalid`, `CodePlaylistGroupInvalid`, `CodeRefManifestInvalid`, `CodeChannelInvalid`.

```go
var coded *dp1.CodedError
if errors.As(err, &coded) {
    _ = coded.Code
}
```

### `dpVersion` (DP-1 §12)

```go
v, err := dp1.ParseDPVersion(p.DPVersion)
if err != nil { /* ... */ }
_ = dp1.WarnMajorMismatch(v, 1) // optional: warn if document major ≠ player major
```

### Signing (`github.com/display-protocol/dp1-go/sign`)

- `sign.PayloadHashString` / `sign.VerifyPayloadHash` — `sha256:<hex>` for `payload_hash` (DP-1 §7.1: strip top-level signature fields, JCS, trailing LF, then SHA-256).
- `sign.SignLegacyEd25519` / `sign.VerifyLegacyEd25519` — v1.0.x `signature: ed25519:<hex>` over that same digest.
- `sign.Ed25519DIDKey` / `sign.Ed25519PublicKeyFromDIDKey` — encode and decode `did:key` for a raw Ed25519 public key (W3C did:key: multicodec ed25519-pub + multibase base58btc). `VerifyMultiEd25519` accepts only this `kid` form.
- `sign.SignMultiEd25519` / `sign.VerifyMultiEd25519` — v1.1+ `signatures[]`: Ed25519 signs the same **32-byte** digest; `SignMultiEd25519` sets `kid` via `Ed25519DIDKey`; `payload_hash` is checked separately. Ed25519 only for now.
- `sign.VerifyMultiSignaturesJSON` — decodes the top-level `signatures` array from raw JSON and verifies each entry (same rules as `VerifyMultiEd25519`); shared by playlist, playlist-group, and channel documents. `sign.VerifyPlaylistSignatures`, `sign.VerifyPlaylistGroupSignatures`, and `sign.VerifyChannelSignatures` are equivalent wrappers for clarity. Returns an error if JSON is invalid, `signatures` is missing or empty (`ErrNoSignatures`), or a signature entry cannot decode; otherwise ok plus failed `playlist.Signature` values (in order). Non-Ed25519 algorithms count as failures.

### Display merge (`github.com/display-protocol/dp1-go/merge`)

Resolution order: defaults → ref manifest controls → item `override` → item-local display fields.

```go
import "github.com/display-protocol/dp1-go/merge"

prefs, err := merge.DisplayForItem(def, refManifest, item)
```

### Extension types (optional)

Shared and extension-specific structs live under `extension/` (for example `extension/playlists` for the playlists overlay, `extension/identity` for `Entity`, `extension/channels` for the channel document type). Prefer `ParseAndValidate*` at the root package for full schema validation.

## Schemas

Normative JSON Schemas are embedded from the spec repo under `internal/schema/` (core v1.1.0 + extensions + a small `overlay` for playlist + playlists-extension `allOf`).

## Testing

```bash
go test ./... -race -count=1
bash scripts/check-coverage.sh 80   # merged module coverage threshold (CI)
```

CI uploads the merged profile to [Codecov](https://codecov.io/gh/display-protocol/dp1-go) after the threshold check. If uploads require authentication, add a `CODECOV_TOKEN` repository secret from [codecov.io](https://codecov.io).

## License

See [LICENSE](LICENSE).
