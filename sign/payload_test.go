package sign

import (
	"encoding/hex"
	"errors"
	"strings"
	"testing"
)

func TestPayloadHashString_idempotent(t *testing.T) {
	t.Parallel()
	raw := []byte(`{"dpVersion":"1.1.0","title":"x","items":[{"source":"https://a"}]}`)
	ph1, err := PayloadHashString(raw)
	if err != nil {
		t.Fatal(err)
	}
	ph2, err := PayloadHashString(raw)
	if err != nil {
		t.Fatal(err)
	}
	if ph1 != ph2 {
		t.Fatalf("expected stable hash, got %q vs %q", ph1, ph2)
	}
	const prefix = "sha256:"
	if !strings.HasPrefix(ph1, prefix) || len(ph1) != len(prefix)+64 {
		t.Fatalf("bad payload hash format: %q", ph1)
	}
	_, err = hex.DecodeString(strings.TrimPrefix(ph1, prefix))
	if err != nil {
		t.Fatalf("suffix must be hex: %v", err)
	}
}

// Two encodings that differ only by signature fields and key order should yield the same payload hash.
func TestPayloadHashString_equivalentAfterStrip(t *testing.T) {
	t.Parallel()
	a := []byte(`{"title":"x","dpVersion":"1.1.0","signature":"ed25519:ab","items":[{"source":"https://z"}],"signatures":[]}`)
	b := []byte(`{"dpVersion":"1.1.0","title":"x","items":[{"source":"https://z"}]}`)
	ha, err := PayloadHashString(a)
	if err != nil {
		t.Fatal(err)
	}
	hb, err := PayloadHashString(b)
	if err != nil {
		t.Fatal(err)
	}
	if ha != hb {
		t.Fatalf("expected equal hashes, got %q vs %q", ha, hb)
	}
}

func TestPayloadHashString_invalidJSON(t *testing.T) {
	t.Parallel()
	for _, raw := range [][]byte{[]byte(`not-json`), []byte(`{`)} {
		_, err := PayloadHashString(raw)
		if err == nil {
			t.Fatalf("expected error for %q", raw)
		}
	}
}

func TestVerifyPayloadHash_ok(t *testing.T) {
	t.Parallel()
	raw := []byte(`{"dpVersion":"1.1.0","title":"x","items":[{"source":"https://a"}]}`)
	want, err := PayloadHashString(raw)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyPayloadHash(raw, want); err != nil {
		t.Fatalf("expected match, got %v", err)
	}
}

func TestVerifyPayloadHash_mismatch(t *testing.T) {
	t.Parallel()
	raw := []byte(`{"dpVersion":"1.1.0","title":"x","items":[{"source":"https://a"}]}`)
	err := VerifyPayloadHash(raw, "sha256:0000000000000000000000000000000000000000000000000000000000000000")
	if err == nil {
		t.Fatal("expected mismatch")
	}
	if !strings.Contains(err.Error(), "payload_hash") {
		t.Fatalf("expected payload_hash mismatch error, got %v", err)
	}
}

func TestVerifyPayloadHash_wrongPrefixStillMismatch(t *testing.T) {
	t.Parallel()
	raw := []byte(`{"dpVersion":"1.1.0","title":"x","items":[{"source":"https://a"}]}`)
	ph, err := PayloadHashString(raw)
	if err != nil {
		t.Fatal(err)
	}
	hexOnly := strings.TrimPrefix(ph, "sha256:")
	err = VerifyPayloadHash(raw, hexOnly)
	if err == nil {
		t.Fatal("expected mismatch")
	}
	if !strings.Contains(err.Error(), "payload_hash") {
		t.Fatalf("expected payload_hash mismatch error, got %v", err)
	}
}

func TestVerifyPayloadHash_invalidJSON(t *testing.T) {
	t.Parallel()
	err := VerifyPayloadHash([]byte(`not json`), "sha256:0000000000000000000000000000000000000000000000000000000000000000")
	if err == nil {
		t.Fatal("expected error")
	}
	if errors.Is(err, ErrSigInvalid) {
		t.Fatal("JSON parse failure should not be reported as ErrSigInvalid")
	}
}
