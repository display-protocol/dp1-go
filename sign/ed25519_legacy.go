package sign

import (
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"strings"
)

const legacyPrefix = "ed25519:"

// VerifyLegacyEd25519 verifies a v1.0.x legacy playlist signature (DP-1 §7.1.2).
// The signed message is the raw 32-byte SHA-256 digest of the §7.1 signing octets (same as v1.1+ multi-sig).
func VerifyLegacyEd25519(rawPlaylistJSON []byte, legacySig string, pub ed25519.PublicKey) error {
	if legacySig == "" {
		return fmt.Errorf("%w: empty legacy signature", ErrSigInvalid)
	}
	if !strings.HasPrefix(legacySig, legacyPrefix) {
		return fmt.Errorf("%w: expected prefix %q", ErrSigInvalid, legacyPrefix)
	}
	hexPart := strings.TrimPrefix(legacySig, legacyPrefix)
	sig, err := hex.DecodeString(hexPart)
	if err != nil {
		return fmt.Errorf("%w: decode hex: %w", ErrSigInvalid, err)
	}
	if len(sig) != ed25519.SignatureSize {
		return fmt.Errorf("%w: bad signature length %d", ErrSigInvalid, len(sig))
	}
	digest, err := signingDigest(rawPlaylistJSON)
	if err != nil {
		return err
	}
	if !ed25519.Verify(pub, digest[:], sig) {
		return ErrSigInvalid
	}
	return nil
}

// SignLegacyEd25519 produces "ed25519:<hex>" over the same digest as PayloadHashString (same pipeline as multi-sig).
func SignLegacyEd25519(rawPlaylistJSON []byte, priv ed25519.PrivateKey) (string, error) {
	digest, err := signingDigest(rawPlaylistJSON)
	if err != nil {
		return "", err
	}
	sig := ed25519.Sign(priv, digest[:])
	return legacyPrefix + hex.EncodeToString(sig), nil
}
