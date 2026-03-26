package sign

import "errors"

// ErrSigInvalid indicates signature verification failed.
var ErrSigInvalid = errors.New("dp1: invalid signature")

// ErrUnsupportedAlg is returned when a signature uses a non-Ed25519 algorithm (not yet implemented).
var ErrUnsupportedAlg = errors.New("dp1: signature algorithm not implemented")

// ErrNoSignatures is returned by [VerifyMultiSignaturesJSON] (and the playlist / playlist-group /
// channel wrappers) when the document has no "signatures" array or it is empty.
var ErrNoSignatures = errors.New("dp1: no signatures in document")
