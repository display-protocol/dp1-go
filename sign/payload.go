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

// PayloadHashString returns "sha256:"+hex(signingDigest(raw)) for signatures[].payload_hash.
func PayloadHashString(raw []byte) (string, error) {
	sum, err := signingDigest(raw)
	if err != nil {
		return "", err
	}
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

// VerifyPayloadHash checks that wantHash equals PayloadHashString(raw).
// A mismatch is returned as a plain error; that sentinel is for Ed25519 verify failure.
func VerifyPayloadHash(raw []byte, wantHash string) error {
	got, err := PayloadHashString(raw)
	if err != nil {
		return err
	}
	if got != wantHash {
		return fmt.Errorf("payload_hash does not match canonical document digest")
	}
	return nil
}
