package sign

import (
	"crypto/ed25519"
	"fmt"

	"github.com/display-protocol/dp1-go/playlist"
)

// Ed25519Signer implements signature creation for the ed25519 algorithm.
// It signs the DP-1 digest with Ed25519 and produces did:key identifiers.
type Ed25519Signer struct {
	privateKey ed25519.PrivateKey
}

// NewEd25519Signer creates an Ed25519Signer from a private key.
func NewEd25519Signer(priv ed25519.PrivateKey) *Ed25519Signer {
	return &Ed25519Signer{privateKey: priv}
}

// Alg returns "ed25519" to match playlist.AlgEd25519.
func (s *Ed25519Signer) Alg() string {
	return playlist.AlgEd25519
}

// Sign creates an Ed25519 signature over the DP-1 signing digest.
//
// Returns:
//   - kid: W3C did:key identifier derived from the public key
//   - sigBytes: 64-byte Ed25519 signature
//   - err: Non-nil if DID derivation fails (should not happen with valid key)
func (s *Ed25519Signer) Sign(digest [32]byte) (string, []byte, error) {
	pub, ok := s.privateKey.Public().(ed25519.PublicKey)
	if !ok {
		return "", nil, fmt.Errorf("ed25519 private key has unexpected public key type %T", s.privateKey.Public())
	}
	kid, err := Ed25519DIDKey(pub)
	if err != nil {
		return "", nil, err
	}
	sig := ed25519.Sign(s.privateKey, digest[:])
	return kid, sig, nil
}
