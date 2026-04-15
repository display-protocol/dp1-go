package sign

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/display-protocol/dp1-go/playlist"
)

// VerifyMultiSignature verifies one signature entry using the appropriate algorithm verifier.
// Supports ed25519 (did:key) and eip191 (did:pkh) algorithms, with additional algorithms
// registered via [RegisterVerifier].
//
// The verification process:
//  1. Validate payload_hash matches the canonical document digest
//  2. Compute the DP-1 signing digest (SHA-256 of JCS + LF)
//  3. Look up the algorithm-specific verifier from the registry
//  4. Decode the base64url signature
//  5. Delegate verification to the algorithm-specific verifier
//
// Returns ErrUnsupportedAlg if sig.Alg is not registered, ErrSigInvalid if signature
// verification fails, or another error for parsing/validation failures.
func VerifyMultiSignature(rawPlaylistJSON []byte, sig playlist.Signature) error {
	// Verify payload_hash assertion
	if err := VerifyPayloadHash(rawPlaylistJSON, sig.PayloadHash); err != nil {
		return err
	}

	// Compute DP-1 signing digest (same for all algorithms)
	digest, err := signingDigest(rawPlaylistJSON)
	if err != nil {
		return err
	}

	// Get algorithm-specific verifier
	verifier, err := GetVerifier(sig.Alg)
	if err != nil {
		return err
	}

	// Decode signature bytes
	sigBytes, err := base64.RawURLEncoding.DecodeString(sig.Sig)
	if err != nil {
		return fmt.Errorf("%w: decode sig: %w", ErrSigInvalid, err)
	}

	// Delegate to algorithm-specific verification
	return verifier.VerifySignature(sig.Kid, sigBytes, digest)
}

// VerifyMultiEd25519 verifies one multi-signature entry when alg is ed25519.
// sig.Kid must be a did:key carrying an Ed25519 public key (same form as [Ed25519DIDKey]); otherwise
// verification is rejected. payload_hash must match PayloadHashString(raw) (assertion only; the hex
// string is not passed to Ed25519). Ed25519 verifies over the same 32-byte digest as legacy §7.1.2.
func VerifyMultiEd25519(rawPlaylistJSON []byte, sig playlist.Signature) error {
	if strings.ToLower(sig.Alg) != playlist.AlgEd25519 {
		return fmt.Errorf("%w: %q", ErrUnsupportedAlg, sig.Alg)
	}
	return VerifyMultiSignature(rawPlaylistJSON, sig)
}

// VerifyMultiSignaturesJSON decodes the top-level "signatures" array from rawSignedJSON and verifies
// each entry in order with [VerifyMultiSignature] (same §7.1 digest and payload_hash rules). The
// wire shape is shared by core playlists, playlist-groups, and channel documents—all use the same
// [playlist.Signature] entries under "signatures".
//
// Returns a non-nil err if rawSignedJSON is not valid JSON, cannot be decoded (including when
// "signatures" is present but not a JSON array of signature objects), or "signatures" is missing or
// empty ([ErrNoSignatures]).
//
// When err is nil: ok is true when every signature verifies. On verification failure, ok is false
// and failed lists each signature object that did not verify, in array order; failed is nil when ok
// is true. For the specific verification error, call [VerifyMultiSignature] with the same raw bytes
// and the returned [playlist.Signature].
//
// Supported algorithms: ed25519 (did:key), eip191 (did:pkh). Entries with unsupported alg values
// are treated as failed verification (not skipped): they are included in failed the same as any
// other failure.
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
		if verr := VerifyMultiSignature(rawSignedJSON, sig); verr != nil {
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

// SignMulti creates a signature using the provided Signer implementation.
// This is the unified signing function that works with any algorithm (ed25519, eip191, etc.).
//
// The signing process:
//  1. Compute the DP-1 signing digest (SHA-256 of JCS + LF)
//  2. Compute payload_hash for the signature assertion field
//  3. Call signer.Sign(digest) to get kid and signature bytes
//  4. Encode signature as base64url and assemble the Signature struct
//
// Returns a complete playlist.Signature ready to be added to a document's "signatures" array.
func SignMulti(rawPlaylistJSON []byte, signer Signer, role string, tsRFC3339 string) (playlist.Signature, error) {
	digest, err := signingDigest(rawPlaylistJSON)
	if err != nil {
		return playlist.Signature{}, err
	}

	payloadHash := "sha256:" + hex.EncodeToString(digest[:])

	kid, sigBytes, err := signer.Sign(digest)
	if err != nil {
		return playlist.Signature{}, err
	}

	return playlist.Signature{
		Alg:         signer.Alg(),
		Kid:         kid,
		Ts:          tsRFC3339,
		PayloadHash: payloadHash,
		Role:        role,
		Sig:         base64.RawURLEncoding.EncodeToString(sigBytes),
	}, nil
}

// SignMultiEd25519 builds a curator/feed-style signature object (testing / feed operators).
// It sets kid to the did:key derived from the private key (W3C did:key, Ed25519) via [Ed25519DIDKey],
// sets payload_hash from the signing digest, and signs that digest with Ed25519 (same as legacy).
func SignMultiEd25519(rawPlaylistJSON []byte, priv ed25519.PrivateKey, role string, tsRFC3339 string) (playlist.Signature, error) {
	return SignMulti(rawPlaylistJSON, NewEd25519Signer(priv), role, tsRFC3339)
}

// SignMultiEIP191 creates an Ethereum personal_sign (EIP-191) signature for DP-1 documents.
// This function is used by curators/feeds operating from Ethereum addresses.
//
// Parameters:
//   - rawPlaylistJSON: The unsigned document JSON
//   - priv: ECDSA private key (secp256k1)
//   - chainID: EVM chain identifier (1=mainnet, 137=Polygon, 42161=Arbitrum, etc.)
//   - role: Signature role (curator, feed, agent, institution, licensor)
//   - tsRFC3339: RFC3339 timestamp (e.g., "2026-04-13T10:00:00Z")
//
// The signature uses did:pkh format: did:pkh:eip155:{chainID}:{address}
//
// See [NewEthereumSigner] for chain ID details and cross-chain replay considerations.
func SignMultiEIP191(rawPlaylistJSON []byte, priv *ecdsa.PrivateKey, chainID int, role string, tsRFC3339 string) (playlist.Signature, error) {
	return SignMulti(rawPlaylistJSON, NewEthereumSigner(priv, chainID), role, tsRFC3339)
}
