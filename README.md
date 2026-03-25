# dp1-go

Go SDK for the [DP-1 protocol](https://github.com/display-protocol/dp1): playlists, playlist-groups (exhibitions), ref manifests, JCS signing payloads (RFC 8785), Ed25519 verification (legacy + multi-signature), and registered JSON Schema extensions.

**Module:** `github.com/display-protocol/dp1-go`

## Requirements

- Go 1.22+

## Install

```bash
go get github.com/display-protocol/dp1-go
```

## Usage

Parse and validate in one step (embedded JSON Schema, draft 2020-12):

```go
import "github.com/display-protocol/dp1-go"

p, err := dp1.ParseAndValidatePlaylist(playlistJSON)
if err != nil {
    // errors.Is(err, dp1.ErrValidation) for schema failures
    return err
}
```

Playlist with the optional **playlists** extension overlay:

```go
p, err := dp1.ParseAndValidatePlaylistWithPlaylistsExtension(data)
```

Signing helpers (`github.com/display-protocol/dp1-go/sign`):

- `sign.PayloadHashString` / `sign.VerifyPayloadHash` — `sha256:<hex>` for `payload_hash` (DP-1 §7.1: JCS after strip, trailing LF, then SHA-256)
- `sign.SignLegacyEd25519` / `sign.VerifyLegacyEd25519` — v1.0.x `signature: ed25519:<hex>` over that same digest
- `sign.SignMultiEd25519` / `sign.VerifyMultiEd25519` — v1.1+ `signatures[]`: Ed25519 signs the same **32-byte** digest; `payload_hash` is checked separately for assertion (Ed25519 only for now)

Display merge order (`github.com/display-protocol/dp1-go/merge`): defaults → ref manifest controls → item `override` → item-local display fields.

## Schemas

Normative JSON Schemas are embedded from the spec repo under `internal/schema/` (core v1.1.0 + extensions + a small `overlay` for playlist + playlists-extension `allOf`).

## Testing

```bash
go test ./... -race
bash scripts/check-coverage.sh 80   # merged module coverage threshold
```

## License

See [LICENSE](LICENSE).
