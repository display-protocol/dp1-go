package sign

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"testing"

	"github.com/display-protocol/dp1-go/playlist"
)

type stubResolver struct {
	pub ed25519.PublicKey
}

func (s stubResolver) Ed25519PublicKey(ctx context.Context, kid string) (ed25519.PublicKey, error) {
	return s.pub, nil
}

type errResolver struct {
	err error
}

func (e errResolver) Ed25519PublicKey(ctx context.Context, kid string) (ed25519.PublicKey, error) {
	return nil, e.err
}

func TestMultiSigRoundTrip(t *testing.T) {
	t.Parallel()
	pub, priv, _ := ed25519.GenerateKey(nil)
	kid := "did:key:z6MkhaXgBZDvotDkL5257faiztiGiC2QtKLGpbnnEGta2doK"
	p := playlist.Playlist{
		DPVersion: "1.1.0",
		Title:     "Multi",
		Items:     []playlist.PlaylistItem{{Source: "https://example.com"}},
	}
	raw, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	sig, err := SignMultiEd25519(raw, priv, kid, playlist.RoleCurator, "2025-01-01T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	res := stubResolver{pub}
	if err := VerifyMultiEd25519(context.Background(), raw, sig, res); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyMultiEd25519_unsupportedAlg(t *testing.T) {
	t.Parallel()
	sig := playlist.Signature{
		Alg:         playlist.AlgEIP191,
		Kid:         "did:key:z",
		Ts:          "2025-01-01T00:00:00Z",
		PayloadHash: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Role:        playlist.RoleFeed,
		Sig:         "abc",
	}
	err := VerifyMultiEd25519(context.Background(), []byte(`{"dpVersion":"1.1.0","title":"x","items":[{"source":"https://a"}]}`), sig, stubResolver{ed25519.PublicKey(make([]byte, 32))})
	if err == nil {
		t.Fatal("expected unsupported alg")
	}
}

func TestVerifyMultiEd25519_resolverError(t *testing.T) {
	t.Parallel()
	_, priv, _ := ed25519.GenerateKey(nil)
	pl := playlist.Playlist{DPVersion: "1.1.0", Title: "x", Items: []playlist.PlaylistItem{{Source: "https://a"}}}
	raw, err := json.Marshal(pl)
	if err != nil {
		t.Fatal(err)
	}
	sig, err := SignMultiEd25519(raw, priv, "did:key:z", playlist.RoleFeed, "2025-01-01T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	err = VerifyMultiEd25519(context.Background(), raw, sig, errResolver{err: errors.New("no key")})
	if err == nil {
		t.Fatal("expected error from resolver")
	}
}

func TestVerifyMultiEd25519_wrongSigBytes(t *testing.T) {
	t.Parallel()
	pub, priv, _ := ed25519.GenerateKey(nil)
	p := playlist.Playlist{
		DPVersion: "1.1.0",
		Title:     "x",
		Items:     []playlist.PlaylistItem{{Source: "https://a"}},
	}
	raw, _ := json.Marshal(p)
	sig, err := SignMultiEd25519(raw, priv, "did:key:z", playlist.RoleFeed, "2025-01-01T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	// Valid-length Ed25519 signature bytes (wrong key/material) so decoding succeeds.
	sig.Sig = base64.RawURLEncoding.EncodeToString(make([]byte, ed25519.SignatureSize))
	err = VerifyMultiEd25519(context.Background(), raw, sig, stubResolver{pub})
	if err == nil {
		t.Fatal("expected verify failure")
	}
}

func TestVerifyMultiWrongHash(t *testing.T) {
	t.Parallel()
	pub, priv, _ := ed25519.GenerateKey(nil)
	p := playlist.Playlist{
		DPVersion: "1.1.0",
		Title:     "x",
		Items:     []playlist.PlaylistItem{{Source: "https://a"}},
	}
	raw, _ := json.Marshal(p)
	sig, err := SignMultiEd25519(raw, priv, "did:key:z", playlist.RoleFeed, "2025-01-01T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	sig.PayloadHash = "sha256:" + "00" + sig.PayloadHash[len(sig.PayloadHash)-62:]
	err = VerifyMultiEd25519(context.Background(), raw, sig, stubResolver{pub})
	if err == nil {
		t.Fatal("expected error")
	}
}
