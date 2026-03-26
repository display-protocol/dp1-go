package sign

import (
	"bytes"
	"crypto/ed25519"
	"errors"
	"testing"

	mb "github.com/multiformats/go-multibase"
)

// Public key from multibase z6Mk… (ed25519-pub multicodec + 32-byte key); used across parse/validate fixtures.
const testVectorDIDKey = "did:key:z6MkhaXgBZDvotDkL5257faiztiGiC2QtKLGpbnnEGta2doK"

func TestEd25519DIDKey_wellKnownVector(t *testing.T) {
	t.Parallel()
	_, data, err := mb.Decode("z6MkhaXgBZDvotDkL5257faiztiGiC2QtKLGpbnnEGta2doK")
	if err != nil {
		t.Fatal(err)
	}
	const edPubMC = 2 // ed25519-pub multicodec (varint 0xed, 0x01)
	if len(data) != edPubMC+ed25519.PublicKeySize {
		t.Fatalf("unexpected multicodec payload len %d", len(data))
	}
	if data[0] != 0xed || data[1] != 0x01 {
		t.Fatalf("unexpected multicodec prefix %x", data[:2])
	}
	pub := ed25519.PublicKey(data[edPubMC:])
	got, err := Ed25519DIDKey(pub)
	if err != nil {
		t.Fatal(err)
	}
	if got != testVectorDIDKey {
		t.Fatalf("Ed25519DIDKey = %q, want %q", got, testVectorDIDKey)
	}
}

func TestEd25519DIDKey_wrongKeyLength(t *testing.T) {
	t.Parallel()
	_, err := Ed25519DIDKey(ed25519.PublicKey(make([]byte, 31)))
	if err == nil {
		t.Fatal("expected error for short public key")
	}
}

func TestEd25519DIDKey_roundTripMultibase(t *testing.T) {
	t.Parallel()
	pub, _, _ := ed25519.GenerateKey(nil)
	did, err := Ed25519DIDKey(pub)
	if err != nil {
		t.Fatal(err)
	}
	const prefix = "did:key:"
	if len(did) <= len(prefix) || did[:len(prefix)] != prefix {
		t.Fatalf("unexpected DID %q", did)
	}
	mbStr := did[len(prefix):]
	enc, data, err := mb.Decode(mbStr)
	if err != nil {
		t.Fatal(err)
	}
	if enc != mb.Base58BTC {
		t.Fatalf("encoding %c, want base58btc", enc)
	}
	const edPubMC = 2
	if len(data) != edPubMC+ed25519.PublicKeySize {
		t.Fatalf("decoded len %d", len(data))
	}
	if !bytes.Equal(data[edPubMC:], pub) {
		t.Fatal("public key mismatch after decode")
	}
}

func TestEd25519PublicKeyFromDIDKey_wellKnownVector(t *testing.T) {
	t.Parallel()
	pub, err := Ed25519PublicKeyFromDIDKey(testVectorDIDKey)
	if err != nil {
		t.Fatal(err)
	}
	roundTrip, err := Ed25519DIDKey(pub)
	if err != nil {
		t.Fatal(err)
	}
	if roundTrip != testVectorDIDKey {
		t.Fatalf("roundTrip = %q", roundTrip)
	}
}

func TestEd25519PublicKeyFromDIDKey_roundTripRandom(t *testing.T) {
	t.Parallel()
	pub, _, _ := ed25519.GenerateKey(nil)
	did, err := Ed25519DIDKey(pub)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Ed25519PublicKeyFromDIDKey(did)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, pub) {
		t.Fatal("public key mismatch")
	}
}

func TestEd25519PublicKeyFromDIDKey_prefixCaseInsensitive(t *testing.T) {
	t.Parallel()
	pub, err := Ed25519PublicKeyFromDIDKey("DiD:KeY:z6MkhaXgBZDvotDkL5257faiztiGiC2QtKLGpbnnEGta2doK")
	if err != nil {
		t.Fatal(err)
	}
	if len(pub) != ed25519.PublicKeySize {
		t.Fatalf("len %d", len(pub))
	}
}

func TestEd25519PublicKeyFromDIDKey_rejectsNonDidKey(t *testing.T) {
	t.Parallel()
	_, err := Ed25519PublicKeyFromDIDKey("did:web:example.com")
	if err == nil || !errors.Is(err, ErrSigInvalid) {
		t.Fatalf("err = %v", err)
	}
}

func TestEd25519PublicKeyFromDIDKey_rejectsWrongMulticodec(t *testing.T) {
	t.Parallel()
	// Valid-length multibase payload but wrong multicodec prefix (not ed25519-pub).
	wrong := make([]byte, 2+ed25519.PublicKeySize)
	wrong[0], wrong[1] = 0x12, 0x20 // sha2-256, not ed25519
	copy(wrong[2:], make([]byte, ed25519.PublicKeySize))
	enc, err := mb.Encode(mb.Base58BTC, wrong)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Ed25519PublicKeyFromDIDKey(didKeyPrefix + enc)
	if err == nil || !errors.Is(err, ErrSigInvalid) {
		t.Fatalf("err = %v", err)
	}
}
