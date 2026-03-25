package sign

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/display-protocol/dp1-go/playlist"
)

// PublicKeyResolver resolves a signature's kid (DID, etc.) to an Ed25519 public key.
type PublicKeyResolver interface {
	Ed25519PublicKey(ctx context.Context, kid string) (ed25519.PublicKey, error)
}

// VerifyMultiEd25519 verifies one multi-signature entry when alg is ed25519.
// payload_hash must match PayloadHashString(raw) (assertion only; the hex string is not passed to Ed25519).
// Ed25519 verifies over the same 32-byte digest as legacy §7.1.2.
func VerifyMultiEd25519(ctx context.Context, rawPlaylistJSON []byte, sig playlist.Signature, resolve PublicKeyResolver) error {
	if strings.ToLower(sig.Alg) != playlist.AlgEd25519 {
		return fmt.Errorf("%w: %q", ErrUnsupportedAlg, sig.Alg)
	}
	if err := VerifyPayloadHash(rawPlaylistJSON, sig.PayloadHash); err != nil {
		return err
	}
	digest, err := signingDigest(rawPlaylistJSON)
	if err != nil {
		return err
	}
	pub, err := resolve.Ed25519PublicKey(ctx, sig.Kid)
	if err != nil {
		return fmt.Errorf("%w: resolve kid: %w", ErrSigInvalid, err)
	}
	rawSig, err := base64.RawURLEncoding.DecodeString(sig.Sig)
	if err != nil {
		return fmt.Errorf("%w: decode sig: %w", ErrSigInvalid, err)
	}
	if len(rawSig) != ed25519.SignatureSize {
		return fmt.Errorf("%w: bad signature length", ErrSigInvalid)
	}
	if !ed25519.Verify(pub, digest[:], rawSig) {
		return ErrSigInvalid
	}
	return nil
}

// SignMultiEd25519 builds a curator/feed-style signature object (testing / feed operators).
// It sets payload_hash from the signing digest and signs that digest with Ed25519 (same as legacy).
func SignMultiEd25519(rawPlaylistJSON []byte, priv ed25519.PrivateKey, kid, role string, tsRFC3339 string) (playlist.Signature, error) {
	digest, err := signingDigest(rawPlaylistJSON)
	if err != nil {
		return playlist.Signature{}, err
	}
	ph := "sha256:" + hex.EncodeToString(digest[:])
	sig := ed25519.Sign(priv, digest[:])
	return playlist.Signature{
		Alg:         playlist.AlgEd25519,
		Kid:         kid,
		Ts:          tsRFC3339,
		PayloadHash: ph,
		Role:        role,
		Sig:         base64.RawURLEncoding.EncodeToString(sig),
	}, nil
}
