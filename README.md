# dp1-go

[![Test](https://github.com/display-protocol/dp1-go/actions/workflows/test.yaml/badge.svg)](https://github.com/display-protocol/dp1-go/actions/workflows/test.yaml?query=branch%3Amain)
[![Lint](https://github.com/display-protocol/dp1-go/actions/workflows/lint.yaml/badge.svg)](https://github.com/display-protocol/dp1-go/actions/workflows/lint.yaml?query=branch%3Amain)
[![codecov](https://codecov.io/gh/display-protocol/dp1-go/graph/badge.svg)](https://codecov.io/gh/display-protocol/dp1-go)

Go SDK for the [DP-1 protocol](https://github.com/display-protocol/dp1): playlists, playlist-groups (exhibitions), ref manifests, JCS signing payloads (RFC 8785), signature verification (Ed25519 + Ethereum EIP-191), and registered JSON Schema extensions.

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

Dynamic playlist items (playlists extension `dynamicQuery`): hydrate `{{placeholders}}` with `playlist.HydrationParams`, fetch the indexer, map response rows, validate each item against core `PlaylistItem`, and append after static items via `(*playlist.Playlist).ResolveDynamicQuery` (pass `*http.Client`, or `nil` for `http.DefaultClient`, and `*playlist.DynamicQueryFetchOptions` or `nil` for HTTPS-only + SSRF-safe defaults). Set `AllowInsecureHTTP` on the options value to allow `http://` and local addresses (for example `httptest`). The same fetch and decode path is available as `playlist.PlaylistItemsFromDynamicQuery(ctx, dq, params, client, opts)` when you only need `[]PlaylistItem`. Use `errors.Is(err, playlist.ErrDynamicQueryEndpointPolicy)` when the outbound URL fails policy checks.

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

The `sign` package implements DP-1 §7.1 signing: strip signature fields → JCS (RFC 8785) → append LF → SHA-256. All algorithms sign the same 32-byte digest. Supports Ed25519 (`did:key`) and Ethereum EIP-191 (`did:pkh`).

#### Common

- `sign.PayloadHashString` / `sign.VerifyPayloadHash` — compute or verify `sha256:<hex>` for `payload_hash` field.

#### Ed25519 (algorithm: `ed25519`)

- `sign.SignMultiEd25519(raw, priv, role, ts)` — create v1.1+ signature with `did:key` kid.
- `sign.VerifyMultiSignature(raw, sig)` — verify any supported algorithm (ed25519, eip191).
- `sign.Ed25519DIDKey` / `sign.Ed25519PublicKeyFromDIDKey` — encode/decode W3C `did:key` for Ed25519 public keys.
- Legacy v1.0.x: `sign.SignLegacyEd25519` / `sign.VerifyLegacyEd25519` — single `signature: ed25519:<hex>` field.

#### Ethereum EIP-191 (algorithm: `eip191`)

- `sign.SignMultiEIP191(raw, priv, chainID, role, ts)` — create signature using Ethereum personal_sign (EIP-191 version 0x45) with `did:pkh` kid.
- `sign.VerifyMultiSignature(raw, sig)` — verify (same function as Ed25519, dispatches by `sig.Alg`).
- `sign.EthereumAddressToDIDPKH(addr, chainID)` / `sign.EthereumAddressFromDIDPKH(kid)` — encode/decode `did:pkh:eip155:{chainID}:{address}` (CAIP-10).
- Works with all EVM chains: Ethereum (1), Polygon (137), Arbitrum (42161), Base (8453), etc.

**Example: Sign with Ethereum**

```go
import (
    "github.com/ethereum/go-ethereum/crypto"
    "github.com/display-protocol/dp1-go/sign"
    "github.com/display-protocol/dp1-go/playlist"
)

priv, _ := crypto.GenerateKey()  // or crypto.HexToECDSA(hexKey)
raw, _ := json.Marshal(playlist)

// Sign for Ethereum mainnet (chainID=1)
sig, err := sign.SignMultiEIP191(raw, priv, 1, playlist.RoleCurator, "2026-04-13T10:00:00Z")
// sig.Alg = "eip191"
// sig.Kid = "did:pkh:eip155:1:0xB9C5714089478a327F09197987f16f9E5d936E8a"

// Verify
err = sign.VerifyMultiSignature(raw, sig)
```

#### Multi-signature verification

- `sign.VerifyMultiSignaturesJSON(raw)` — decode `signatures[]` array and verify all entries; returns `(ok, failed, err)`.
- `sign.VerifyPlaylistSignatures` / `sign.VerifyPlaylistGroupSignatures` / `sign.VerifyChannelSignatures` — equivalent wrappers for clarity.
- Documents can mix Ed25519 and Ethereum signatures; each is verified independently.

**Replay protections:** Cross-document replay prevented by `payload_hash`. Cross-chain and temporal replay not enforced (see package docs).

### Display merge (`github.com/display-protocol/dp1-go/merge`)

Resolution order: defaults → ref manifest controls → item `override` → item-local display fields.

```go
import "github.com/display-protocol/dp1-go/merge"

prefs, err := merge.DisplayForItem(def, refManifest, item)
```

### Extension types (optional)

Shared and extension-specific structs live under `extension/` (for example `extension/playlists` for the playlists overlay—`DynamicQuery`, experimental `Note` on `playlist.Playlist` and `playlist.PlaylistItem`, `extension/identity` for `Entity`, `extension/channels` for the channel document type). Prefer `ParseAndValidate*` at the root package for full schema validation.

## Schemas

Normative JSON Schemas are embedded from the spec repo under `internal/schema/` (core v1.1.0 + extensions, including `extensions/playlists/schema.json` with optional `note` / per-item `note` overlays, and `playlist_with_extension.json` for full playlist + playlists-extension validation).

## Testing

```bash
go test ./... -race -count=1
bash scripts/check-coverage.sh 80   # merged module coverage threshold (CI)
```

CI uploads the merged profile to [Codecov](https://codecov.io/gh/display-protocol/dp1-go) after the threshold check. If uploads require authentication, add a `CODECOV_TOKEN` repository secret from [codecov.io](https://codecov.io).

## License

See [LICENSE](LICENSE).
