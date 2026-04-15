package sign

import (
	"crypto/ed25519"
	"fmt"

	"github.com/display-protocol/dp1-go/playlist"
)

// Ed25519Verifier implements signature verification for the ed25519 algorithm.
// It uses did:key identifiers (W3C did:key with Ed25519 multicodec) and verifies
// Ed25519 signatures over the DP-1 signing digest.
type Ed25519Verifier struct{}

// Alg returns "ed25519" to match playlist.AlgEd25519.
func (v *Ed25519Verifier) Alg() string {
	return playlist.AlgEd25519
}

// VerifySignature verifies an Ed25519 signature.
//
// The kid must be a did:key with Ed25519 public key (parsed via Ed25519PublicKeyFromDIDKey).
// The sigBytes must be exactly 64 bytes (ed25519.SignatureSize).
// The digest is the 32-byte DP-1 signing hash that Ed25519 verifies against.
//
// Returns ErrSigInvalid if the signature does not verify, or another error if kid parsing fails.
func (v *Ed25519Verifier) VerifySignature(kid string, sigBytes []byte, digest [32]byte) error {
	pub, err := Ed25519PublicKeyFromDIDKey(kid)
	if err != nil {
		return err
	}
	if len(sigBytes) != ed25519.SignatureSize {
		return fmt.Errorf("%w: ed25519 signature must be %d bytes, got %d", ErrSigInvalid, ed25519.SignatureSize, len(sigBytes))
	}
	if !ed25519.Verify(pub, digest[:], sigBytes) {
		return ErrSigInvalid
	}
	return nil
}

func init() {
	RegisterVerifier(&Ed25519Verifier{})
}
