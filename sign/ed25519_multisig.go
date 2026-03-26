package sign

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/display-protocol/dp1-go/playlist"
)

// VerifyMultiEd25519 verifies one multi-signature entry when alg is ed25519.
// sig.Kid must be a did:key carrying an Ed25519 public key (same form as [Ed25519DIDKey]); otherwise
// verification is rejected. payload_hash must match PayloadHashString(raw) (assertion only; the hex
// string is not passed to Ed25519). Ed25519 verifies over the same 32-byte digest as legacy §7.1.2.
func VerifyMultiEd25519(rawPlaylistJSON []byte, sig playlist.Signature) error {
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
	pub, err := Ed25519PublicKeyFromDIDKey(sig.Kid)
	if err != nil {
		return err
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

// VerifyMultiSignaturesJSON decodes the top-level "signatures" array from rawSignedJSON and verifies
// each entry in order with [VerifyMultiEd25519] (same §7.1 digest and payload_hash rules). The
// wire shape is shared by core playlists, playlist-groups, and channel documents—all use the same
// [playlist.Signature] entries under "signatures".
//
// Returns a non-nil err if rawSignedJSON is not valid JSON, cannot be decoded (including when
// "signatures" is present but not a JSON array of signature objects), or "signatures" is missing or
// empty ([ErrNoSignatures]).
//
// When err is nil: ok is true when every signature verifies. On verification failure, ok is false
// and failed lists each signature object that did not verify, in array order; failed is nil when ok
// is true. For the specific verification error, call [VerifyMultiEd25519] with the same raw bytes
// and the returned [playlist.Signature].
//
// Only Ed25519 is implemented in this package. Entries with any other alg value are treated as
// failed verification (not skipped): they are included in failed the same as any other failure.
//
// Prefer [VerifyPlaylistSignatures], [VerifyPlaylistGroupSignatures], or [VerifyChannelSignatures]
// at call sites when the document type is known, for clarity; they delegate here.
func VerifyMultiSignaturesJSON(rawSignedJSON []byte) (ok bool, failed []playlist.Signature, err error) {
	var envelope struct {
		Signatures []playlist.Signature `json:"signatures"`
	}
	if err := json.Unmarshal(rawSignedJSON, &envelope); err != nil {
		return false, nil, err
	}
	if len(envelope.Signatures) == 0 {
		return false, nil, ErrNoSignatures
	}
	for _, sig := range envelope.Signatures {
		if verr := VerifyMultiEd25519(rawSignedJSON, sig); verr != nil {
			failed = append(failed, sig)
		}
	}
	if len(failed) == 0 {
		return true, nil, nil
	}
	return false, failed, nil
}

// VerifyPlaylistSignatures verifies v1.1+ signatures in a core playlist JSON document. It is
// equivalent to [VerifyMultiSignaturesJSON].
func VerifyPlaylistSignatures(rawSignedJSON []byte) (ok bool, failed []playlist.Signature, err error) {
	return VerifyMultiSignaturesJSON(rawSignedJSON)
}

// VerifyPlaylistGroupSignatures verifies v1.1+ signatures in a playlist-group JSON document. It is
// equivalent to [VerifyMultiSignaturesJSON].
func VerifyPlaylistGroupSignatures(rawSignedJSON []byte) (ok bool, failed []playlist.Signature, err error) {
	return VerifyMultiSignaturesJSON(rawSignedJSON)
}

// VerifyChannelSignatures verifies v1.1+ signatures in a channel extension JSON document. It is
// equivalent to [VerifyMultiSignaturesJSON].
func VerifyChannelSignatures(rawSignedJSON []byte) (ok bool, failed []playlist.Signature, err error) {
	return VerifyMultiSignaturesJSON(rawSignedJSON)
}

// SignMultiEd25519 builds a curator/feed-style signature object (testing / feed operators).
// It sets kid to the did:key derived from the private key (W3C did:key, Ed25519) via [Ed25519DIDKey],
// sets payload_hash from the signing digest, and signs that digest with Ed25519 (same as legacy).
func SignMultiEd25519(rawPlaylistJSON []byte, priv ed25519.PrivateKey, role string, tsRFC3339 string) (playlist.Signature, error) {
	pub, ok := priv.Public().(ed25519.PublicKey)
	if !ok {
		return playlist.Signature{}, fmt.Errorf("ed25519 private key has unexpected public key type %T", priv.Public())
	}
	kid, err := Ed25519DIDKey(pub)
	if err != nil {
		return playlist.Signature{}, err
	}
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
