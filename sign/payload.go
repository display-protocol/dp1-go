package sign

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	jcspkg "github.com/display-protocol/dp1-go/jcs"
)

// stripSignatureFields returns JSON identical to raw but without top-level
// "signature" or "signatures" keys (DP-1 §7.1).
func stripSignatureFields(raw []byte) ([]byte, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("strip signatures: %w", err)
	}
	delete(m, "signature")
	delete(m, "signatures")
	out, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("strip signatures: %w", err)
	}
	return out, nil
}

// canonicalPayload is JCS( strip(signature fields)(raw) ) — UTF-8, no trailing LF.
func canonicalPayload(raw []byte) ([]byte, error) {
	stripped, err := stripSignatureFields(raw)
	if err != nil {
		return nil, err
	}
	return jcspkg.Transform(stripped)
}

// signingMessage is JCS(strip) + LF (DP-1 §7.1 octets hashed for Ed25519).
func signingMessage(raw []byte) ([]byte, error) {
	canon, err := canonicalPayload(raw)
	if err != nil {
		return nil, err
	}
	out := make([]byte, len(canon)+1)
	copy(out, canon)
	out[len(canon)] = '\n'
	return out, nil
}

// signingDigest is SHA-256(signingMessage(raw)); Ed25519 signs these 32 bytes (legacy + v1.1 multi-sig).
func signingDigest(raw []byte) ([32]byte, error) {
	var zero [32]byte
	msg, err := signingMessage(raw)
	if err != nil {
		return zero, err
	}
	return sha256.Sum256(msg), nil
}

// documentDigest is the single DP-1 §7.1 value that every signature entry in one document is
// checked against: the 32-byte digest of JCS(strip(raw))+LF, together with its "sha256:<hex>"
// rendering for the payload_hash assertion.
//
// It exists so verifying a document costs one canonicalization no matter how many entries its
// "signatures" array carries. The digest is computed over the document with the top-level
// "signature" and "signatures" keys stripped, so it is by construction independent of which entry
// is being verified — the same 32 bytes for all of them. Before this type, VerifyMultiSignature
// canonicalized twice per entry (once through VerifyPayloadHash, once for the digest it passes to
// the verifier) and VerifyMultiSignaturesJSON repeated that inside its loop. On an 8.5 MiB Feral
// File playlist one canonicalization is ~200 ms on a laptop and considerably more on an FF1
// device, so the repetition, not the cryptography, was what made large documents unverifiable
// there. Keep the computation in one place: callers that need the digest and the payload_hash
// string should take both from one documentDigest rather than recomputing either.
type documentDigest struct {
	sum  [32]byte
	hash string // "sha256:" + hex(sum) — the payload_hash wire form
}

// newDocumentDigest canonicalizes raw once (DP-1 §7.1) and keeps both forms of the result.
func newDocumentDigest(raw []byte) (documentDigest, error) {
	sum, err := signingDigest(raw)
	if err != nil {
		return documentDigest{}, err
	}
	return documentDigest{sum: sum, hash: "sha256:" + hex.EncodeToString(sum[:])}, nil
}

// verifyPayloadHash checks a signature entry's payload_hash assertion against this document's
// digest. It is a string compare, not a hash: the work is already done.
func (d documentDigest) verifyPayloadHash(wantHash string) error {
	if d.hash != wantHash {
		return ErrPayloadHashMismatch
	}
	return nil
}

// PayloadHashString returns "sha256:"+hex(signingDigest(raw)) for signatures[].payload_hash.
func PayloadHashString(raw []byte) (string, error) {
	d, err := newDocumentDigest(raw)
	if err != nil {
		return "", err
	}
	return d.hash, nil
}

// VerifyPayloadHash checks that wantHash equals PayloadHashString(raw). A mismatch is
// [ErrPayloadHashMismatch]; [ErrSigInvalid] is reserved for signature verification failure.
//
// This canonicalizes raw on every call. Code verifying several entries of one document should use
// [VerifyMultiSignaturesJSON] or [VerifyMultiSignaturesJSONDetailed], which canonicalize once.
func VerifyPayloadHash(raw []byte, wantHash string) error {
	d, err := newDocumentDigest(raw)
	if err != nil {
		return err
	}
	return d.verifyPayloadHash(wantHash)
}
