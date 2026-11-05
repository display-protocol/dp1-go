package sign

import "errors"

// ErrSigInvalid indicates signature verification failed.
var ErrSigInvalid = errors.New("dp1: invalid signature")

// ErrUnsupportedAlg is returned when a signature uses a non-Ed25519 algorithm (not yet implemented).
var ErrUnsupportedAlg = errors.New("dp1: signature algorithm not implemented")

// ErrNoSignatures is returned by [VerifyMultiSignaturesJSON] (and the playlist / playlist-group /
// channel wrappers) when the document has no "signatures" array or it is empty.
var ErrNoSignatures = errors.New("dp1: no signatures in document")

// ErrPayloadHashMismatch is returned when a signature entry's payload_hash does not match the
// canonical document digest (DP-1 §7.1) — the document is not the one that was signed. It is a
// distinct failure from [ErrSigInvalid], which means the signature itself did not verify over a
// document whose digest did match.
//
// Unlike the sentinels above it carries no "dp1: " prefix. Its text predates it being a sentinel,
// and consumers (feral-controld's sigverify among them) classify this failure with
// strings.Contains(err.Error(), "payload_hash") because there was nothing to match on with
// errors.Is. Keep the message byte-identical; new callers should use errors.Is instead.
var ErrPayloadHashMismatch = errors.New("payload_hash does not match canonical document digest")
