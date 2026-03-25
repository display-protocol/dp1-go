package sign

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"

	"github.com/display-protocol/dp1-go/playlist"
)

func TestLegacySignVerifyRoundTrip(t *testing.T) {
	t.Parallel()
	pub, priv, _ := ed25519.GenerateKey(nil)
	orig := playlist.Playlist{
		DPVersion: "1.0.0",
		Title:     "Test",
		Items: []playlist.PlaylistItem{
			{Source: "https://example.com/a.html"},
		},
	}
	unsigned, err := json.Marshal(orig)
	if err != nil {
		t.Fatal(err)
	}
	legacy, err := SignLegacyEd25519(unsigned, priv)
	if err != nil {
		t.Fatal(err)
	}
	orig.Signature = legacy
	signed, err := json.Marshal(orig)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyLegacyEd25519(signed, legacy, pub); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyLegacyEd25519_errors(t *testing.T) {
	t.Parallel()
	pub, _, _ := ed25519.GenerateKey(nil)
	raw := []byte(`{}`)
	if err := VerifyLegacyEd25519(raw, "", pub); err == nil {
		t.Fatal("expected error for empty sig")
	}
	if err := VerifyLegacyEd25519(raw, "wrong:ab", pub); err == nil {
		t.Fatal("expected error for bad prefix")
	}
	if err := VerifyLegacyEd25519(raw, "ed25519:zz", pub); err == nil {
		t.Fatal("expected error for bad hex")
	}
}

func TestVerifyLegacyEd25519_wrongSigLen(t *testing.T) {
	t.Parallel()
	pub, _, _ := ed25519.GenerateKey(nil)
	shortSig := "ed25519:" + strings.Repeat("ab", 20) // not 64 bytes when decoded
	raw := []byte(`{"dpVersion":"1.0.0","title":"x","items":[{"source":"https://a"}]}`)
	if err := VerifyLegacyEd25519(raw, shortSig, pub); err == nil {
		t.Fatal("expected error")
	}
}

func TestVerifyLegacyEd25519_badSignatureBytes(t *testing.T) {
	t.Parallel()
	pub, priv, _ := ed25519.GenerateKey(nil)
	raw := []byte(`{"dpVersion":"1.0.0","title":"x","items":[{"source":"https://a"}]}`)
	leg, err := SignLegacyEd25519(raw, priv)
	if err != nil {
		t.Fatal(err)
	}
	hexPart := strings.TrimPrefix(leg, "ed25519:")
	b, _ := hex.DecodeString(hexPart)
	b[0] ^= 0xff
	bad := "ed25519:" + hex.EncodeToString(b)
	if err := VerifyLegacyEd25519(raw, bad, pub); err == nil {
		t.Fatal("expected verify failure")
	}
}
