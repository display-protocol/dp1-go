package sign

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"encoding/base64"
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
//  1. Compute the DP-1 signing digest once (SHA-256 of JCS + LF)
//  2. Validate payload_hash matches that digest
//  3. Look up the algorithm-specific verifier from the registry
//  4. Decode the base64url signature
//  5. Delegate verification to the algorithm-specific verifier
//
// Returns [ErrPayloadHashMismatch] if the entry asserts a different document,
// [ErrUnsupportedAlg] if sig.Alg is not registered, [ErrSigInvalid] if signature verification
// fails, or another error for parsing/validation failures.
//
// This canonicalizes rawPlaylistJSON on every call, so verifying N entries of one document
// through it costs N canonicalizations. To verify a whole document's "signatures" array, use
// [VerifyMultiSignaturesJSON] or [VerifyMultiSignaturesJSONDetailed] instead: they canonicalize
// once and then check each entry against that one digest.
func VerifyMultiSignature(rawPlaylistJSON []byte, sig playlist.Signature) error {
	// One canonicalization yields both the payload_hash string and the 32 bytes the verifier
	// signs over; this function used to compute the same digest twice.
	digest, err := newDocumentDigest(rawPlaylistJSON)
	if err != nil {
		return err
	}
	return verifyEntryAgainstDigest(digest, sig)
}

// verifyEntryAgainstDigest performs the per-entry half of verification against a digest that has
// already been computed for the document. Everything here is cheap relative to canonicalization:
// a string compare, a registry lookup, a base64 decode and one signature check.
//
// The order of the checks is part of the public contract and must not be reordered: payload_hash
// first (it says the entry is about a different document, which is not a signature failure), then
// the algorithm lookup, then the signature itself.
func verifyEntryAgainstDigest(digest documentDigest, sig playlist.Signature) error {
	// Verify payload_hash assertion
	if err := digest.verifyPayloadHash(sig.PayloadHash); err != nil {
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
	return verifier.VerifySignature(sig.Kid, sigBytes, digest.sum)
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
// is true.
//
// This function discards the per-entry errors. If you need to know *why* each entry failed, call
// [VerifyMultiSignaturesJSONDetailed] instead — it does the same single canonicalization and
// returns every entry's own error. Do not re-run [VerifyMultiSignature] over the returned failed
// entries to recover their errors: that pays for one whole-document canonicalization per entry,
// which is the cost this pair of functions exists to avoid.
//
// Supported algorithms: ed25519 (did:key), eip191 (did:pkh). Entries with unsupported alg values
// are treated as failed verification (not skipped): they are included in failed the same as any
// other failure.
//
// Prefer [VerifyPlaylistSignatures], [VerifyPlaylistGroupSignatures], or [VerifyChannelSignatures]
// at call sites when the document type is known, for clarity; they delegate here.
func VerifyMultiSignaturesJSON(rawSignedJSON []byte) (ok bool, failed []playlist.Signature, err error) {
	results, err := VerifyMultiSignaturesJSONDetailed(rawSignedJSON)
	if err != nil {
		return false, nil, err
	}
	for _, r := range results {
		if r.Err != nil {
			failed = append(failed, r.Signature)
		}
	}
	if len(failed) == 0 {
		return true, nil, nil
	}
	return false, failed, nil
}

// SignatureResult is one entry's outcome from [VerifyMultiSignaturesJSONDetailed].
type SignatureResult struct {
	// Index is the entry's position in the document's "signatures" array, which is also its
	// position in the returned slice. It is carried on the result so a single result stays
	// self-describing once a caller has filtered or logged it away from the slice.
	Index int

	// Signature is the entry as decoded from the document.
	Signature playlist.Signature

	// Err is nil when the entry verified; otherwise it is exactly what
	// [VerifyMultiSignature] would have returned for this entry and document — one of
	// [ErrPayloadHashMismatch], [ErrUnsupportedAlg], [ErrSigInvalid] (wrapping the base64
	// decode or key-length failure where there is one), or a kid parsing error. Classify it
	// with errors.Is.
	Err error
}

// OK reports whether this entry verified.
func (r SignatureResult) OK() bool { return r.Err == nil }

// VerifyMultiSignaturesJSONDetailed verifies every entry in a document's top-level "signatures"
// array and returns each entry's own outcome, in array order. It is the detailed twin of
// [VerifyMultiSignaturesJSON]: same documents, same decoding, same per-entry rules and the same
// single canonicalization of the document — it only reports more.
//
// Use it when a caller has to say which entry failed and why: labeling a failure from
// [VerifyMultiSignaturesJSON]'s failed slice would otherwise mean re-verifying each entry, and
// each of those re-verifications canonicalizes the whole document again. A document with N
// entries costs one canonicalization here however large N is.
//
// Returns a non-nil err, and nil results, under exactly the conditions
// [VerifyMultiSignaturesJSON] returns one: rawSignedJSON is not valid JSON, "signatures" cannot be
// decoded as an array of signature objects, or it is missing or empty ([ErrNoSignatures]).
// Otherwise results has one entry per signature, and the document verifies when every
// [SignatureResult.OK] is true.
func VerifyMultiSignaturesJSONDetailed(rawSignedJSON []byte) ([]SignatureResult, error) {
	var envelope struct {
		Signatures []playlist.Signature `json:"signatures"`
	}
	if err := json.Unmarshal(rawSignedJSON, &envelope); err != nil {
		return nil, err
	}
	if len(envelope.Signatures) == 0 {
		return nil, ErrNoSignatures
	}

	// The one canonicalization. DP-1 §7.1 strips the top-level "signature" and "signatures" keys
	// before canonicalizing, so this digest does not depend on any entry: it is the same 32 bytes
	// for all of them, whatever the array holds.
	digest, digestErr := newDocumentDigest(rawSignedJSON)

	results := make([]SignatureResult, len(envelope.Signatures))
	for i, sig := range envelope.Signatures {
		// A failure to compute the document digest is reported on every entry rather than as
		// the call's error, so each entry's Err stays exactly what VerifyMultiSignature would
		// return for it, and VerifyMultiSignaturesJSON's (ok, failed) contract is unchanged.
		// Unreachable in practice: the decode above already proved rawSignedJSON is a JSON
		// object, which is the only thing stripping and JCS can fail on.
		err := digestErr
		if err == nil {
			err = verifyEntryAgainstDigest(digest, sig)
		}
		results[i] = SignatureResult{Index: i, Signature: sig, Err: err}
	}
	return results, nil
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
//  1. Compute the DP-1 signing digest once (SHA-256 of JCS + LF)
//  2. Take payload_hash for the signature assertion field from that same digest
//  3. Call signer.Sign(digest) to get kid and signature bytes
//  4. Encode signature as base64url and assemble the Signature struct
//
// Returns a complete playlist.Signature ready to be added to a document's "signatures" array.
func SignMulti(rawPlaylistJSON []byte, signer Signer, role string, tsRFC3339 string) (playlist.Signature, error) {
	digest, err := newDocumentDigest(rawPlaylistJSON)
	if err != nil {
		return playlist.Signature{}, err
	}

	kid, sigBytes, err := signer.Sign(digest.sum)
	if err != nil {
		return playlist.Signature{}, err
	}

	return playlist.Signature{
		Alg:         signer.Alg(),
		Kid:         kid,
		Ts:          tsRFC3339,
		PayloadHash: digest.hash,
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
