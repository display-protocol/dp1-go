package sign

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/display-protocol/dp1-go/playlist"
)

func TestMultiSigRoundTrip(t *testing.T) {
	t.Parallel()
	pub, priv, _ := ed25519.GenerateKey(nil)
	wantKid, err := Ed25519DIDKey(pub)
	if err != nil {
		t.Fatal(err)
	}
	p := playlist.Playlist{
		DPVersion: "1.1.0",
		Title:     "Multi",
		Items:     []playlist.PlaylistItem{{Source: "https://example.com"}},
	}
	raw, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	sig, err := SignMultiEd25519(raw, priv, playlist.RoleCurator, "2025-01-01T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	if sig.Kid != wantKid {
		t.Fatalf("sig.Kid = %q, want %q", sig.Kid, wantKid)
	}
	if err := VerifyMultiEd25519(raw, sig); err != nil {
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
	err := VerifyMultiEd25519([]byte(`{"dpVersion":"1.1.0","title":"x","items":[{"source":"https://a"}]}`), sig)
	if err == nil {
		t.Fatal("expected unsupported alg")
	}
	if !errors.Is(err, ErrUnsupportedAlg) {
		t.Fatalf("err = %v", err)
	}
}

func TestVerifyMultiEd25519_kidNotDidKey(t *testing.T) {
	t.Parallel()
	_, priv, _ := ed25519.GenerateKey(nil)
	pl := playlist.Playlist{DPVersion: "1.1.0", Title: "x", Items: []playlist.PlaylistItem{{Source: "https://a"}}}
	raw, err := json.Marshal(pl)
	if err != nil {
		t.Fatal(err)
	}
	sig, err := SignMultiEd25519(raw, priv, playlist.RoleFeed, "2025-01-01T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	sig.Kid = "did:web:example.com#key1"
	err = VerifyMultiEd25519(raw, sig)
	if err == nil {
		t.Fatal("expected error for non-did:key kid")
	}
	if errors.Is(err, ErrSigInvalid) {
		t.Fatalf("kid parse error must not be ErrSigInvalid; got %v", err)
	}
	if !strings.Contains(err.Error(), "did:key") {
		t.Fatalf("err = %v", err)
	}
}

func TestVerifyMultiEd25519_wrongSigBytes(t *testing.T) {
	t.Parallel()
	_, priv, _ := ed25519.GenerateKey(nil)
	p := playlist.Playlist{
		DPVersion: "1.1.0",
		Title:     "x",
		Items:     []playlist.PlaylistItem{{Source: "https://a"}},
	}
	raw, _ := json.Marshal(p)
	sig, err := SignMultiEd25519(raw, priv, playlist.RoleFeed, "2025-01-01T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	// Valid-length Ed25519 signature bytes (wrong key/material) so decoding succeeds.
	sig.Sig = base64.RawURLEncoding.EncodeToString(make([]byte, ed25519.SignatureSize))
	err = VerifyMultiEd25519(raw, sig)
	if err == nil {
		t.Fatal("expected verify failure")
	}
}

func TestVerifyMultiSignaturesJSON_wrappersMatch(t *testing.T) {
	t.Parallel()
	raw := []byte(`{"signatures":[]}`)
	a, fa, ea := VerifyMultiSignaturesJSON(raw)
	b, fb, eb := VerifyPlaylistSignatures(raw)
	c, fc, ec := VerifyPlaylistGroupSignatures(raw)
	d, fd, ed := VerifyChannelSignatures(raw)
	if a != b || a != c || a != d || !errors.Is(ea, eb) || !errors.Is(ea, ec) || !errors.Is(ea, ed) {
		t.Fatalf("mismatch: %v %v %v %v / %v %v %v %v", ea, eb, ec, ed, a, b, c, d)
	}
	if len(fa) != len(fb) || len(fa) != len(fc) || len(fa) != len(fd) {
		t.Fatal("failed slice len mismatch")
	}
}

func TestVerifyPlaylistSignatures_empty(t *testing.T) {
	t.Parallel()
	t.Run("missing", func(t *testing.T) {
		t.Parallel()
		ok, failed, err := VerifyPlaylistSignatures([]byte(`{"dpVersion":"1.1.0","title":"x","items":[]}`))
		if ok || failed != nil || !errors.Is(err, ErrNoSignatures) {
			t.Fatalf("err=%v ok=%v failed=%v", err, ok, failed)
		}
	})
	t.Run("empty_array", func(t *testing.T) {
		t.Parallel()
		ok, failed, err := VerifyPlaylistSignatures([]byte(`{"signatures":[]}`))
		if ok || failed != nil || !errors.Is(err, ErrNoSignatures) {
			t.Fatalf("err=%v ok=%v failed=%v", err, ok, failed)
		}
	})
}

func TestVerifyPlaylistSignatures_invalidJSON(t *testing.T) {
	t.Parallel()
	_, _, err := VerifyPlaylistSignatures([]byte(`{`))
	if err == nil {
		t.Fatal("expected JSON error")
	}
}

func TestVerifyPlaylistSignatures_allValid(t *testing.T) {
	t.Parallel()
	_, priv, _ := ed25519.GenerateKey(nil)
	p := playlist.Playlist{
		DPVersion: "1.1.0",
		Title:     "x",
		Items:     []playlist.PlaylistItem{{Source: "https://a"}},
	}
	body, _ := json.Marshal(p)
	sig, err := SignMultiEd25519(body, priv, playlist.RoleCurator, "2025-01-01T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	p.Signatures = []playlist.Signature{sig}
	raw, _ := json.Marshal(p)
	ok, failed, err := VerifyPlaylistSignatures(raw)
	if err != nil || !ok || failed != nil {
		t.Fatalf("err=%v ok=%v failed=%v", err, ok, failed)
	}
}

func TestVerifyPlaylistSignatures_unsupportedAlg(t *testing.T) {
	t.Parallel()
	want := playlist.Signature{
		Alg:         playlist.AlgEIP191,
		Kid:         "did:key:z",
		Ts:          "2025-01-01T00:00:00Z",
		PayloadHash: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Role:        playlist.RoleFeed,
		Sig:         "abc",
	}
	doc := playlist.Playlist{
		DPVersion:  "1.1.0",
		Title:      "x",
		Items:      []playlist.PlaylistItem{{Source: "https://a"}},
		Signatures: []playlist.Signature{want},
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	ok, failed, err := VerifyPlaylistSignatures(raw)
	if err != nil || ok || len(failed) != 1 {
		t.Fatalf("err=%v ok=%v failed=%v", err, ok, failed)
	}
	if failed[0] != want {
		t.Fatalf("failed sig = %+v, want %+v", failed[0], want)
	}
	if err := VerifyMultiEd25519(raw, failed[0]); !errors.Is(err, ErrUnsupportedAlg) {
		t.Fatalf("VerifyMultiEd25519 err = %v", err)
	}
}

func TestVerifyPlaylistSignatures_firstOkSecondBad(t *testing.T) {
	t.Parallel()
	_, priv, _ := ed25519.GenerateKey(nil)
	p := playlist.Playlist{
		DPVersion: "1.1.0",
		Title:     "x",
		Items:     []playlist.PlaylistItem{{Source: "https://a"}},
	}
	raw, _ := json.Marshal(p)
	good, err := SignMultiEd25519(raw, priv, playlist.RoleCurator, "2025-01-01T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	bad := good
	bad.Sig = base64.RawURLEncoding.EncodeToString(make([]byte, ed25519.SignatureSize))
	p.Signatures = []playlist.Signature{good, bad}
	raw, _ = json.Marshal(p)
	ok, failed, err := VerifyPlaylistSignatures(raw)
	if err != nil || ok || len(failed) != 1 {
		t.Fatalf("err=%v ok=%v failed=%v", err, ok, failed)
	}
	if failed[0] != bad {
		t.Fatalf("failed = %+v, want %+v", failed[0], bad)
	}
}

func TestVerifyMultiWrongHash(t *testing.T) {
	t.Parallel()
	_, priv, _ := ed25519.GenerateKey(nil)
	p := playlist.Playlist{
		DPVersion: "1.1.0",
		Title:     "x",
		Items:     []playlist.PlaylistItem{{Source: "https://a"}},
	}
	raw, _ := json.Marshal(p)
	sig, err := SignMultiEd25519(raw, priv, playlist.RoleFeed, "2025-01-01T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	sig.PayloadHash = "sha256:" + "00" + sig.PayloadHash[len(sig.PayloadHash)-62:]
	err = VerifyMultiEd25519(raw, sig)
	if err == nil {
		t.Fatal("expected error")
	}
}
