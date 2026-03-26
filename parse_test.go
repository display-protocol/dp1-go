package dp1_test

import (
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/display-protocol/dp1-go"
	"github.com/display-protocol/dp1-go/extension/channels"
	"github.com/display-protocol/dp1-go/extension/identity"
	"github.com/display-protocol/dp1-go/playlist"
	"github.com/display-protocol/dp1-go/playlistgroup"
	"github.com/display-protocol/dp1-go/refmanifest"
	"github.com/display-protocol/dp1-go/sign"
)

// --- ParseAndValidate: playlists ---

func TestParseAndValidatePlaylist_legacySigned(t *testing.T) {
	t.Parallel()
	pub, priv, _ := ed25519.GenerateKey(nil)
	core := playlist.Playlist{
		DPVersion: "1.0.0",
		Title:     "Hello",
		Items: []playlist.PlaylistItem{
			{Source: "https://example.com/work.html"},
		},
	}
	raw, err := json.Marshal(core)
	if err != nil {
		t.Fatal(err)
	}
	legacy, err := sign.SignLegacyEd25519(raw, priv)
	if err != nil {
		t.Fatal(err)
	}
	core.Signature = legacy
	signed, err := json.Marshal(core)
	if err != nil {
		t.Fatal(err)
	}
	p, err := dp1.ParseAndValidatePlaylist(signed)
	if err != nil {
		t.Fatal(err)
	}
	if p.Title != "Hello" {
		t.Fatal(p.Title)
	}
	if err := sign.VerifyLegacyEd25519(signed, p.Signature, pub); err != nil {
		t.Fatal(err)
	}
}

func TestParseAndValidatePlaylist_multiSigned(t *testing.T) {
	t.Parallel()
	_, priv, _ := ed25519.GenerateKey(nil)
	pl := playlist.Playlist{
		DPVersion: "1.1.0",
		Title:     "M",
		Items:     []playlist.PlaylistItem{{Source: "https://a"}},
	}
	body, _ := json.Marshal(pl)
	sig, err := sign.SignMultiEd25519(body, priv, playlist.RoleCurator, "2025-06-01T12:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	pl.Signatures = []playlist.Signature{sig}
	signed, _ := json.Marshal(pl)
	out, err := dp1.ParseAndValidatePlaylist(signed)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Signatures) != 1 {
		t.Fatal(out.Signatures)
	}
}

func TestParseAndValidatePlaylist_schemaRejectsInvalidDoc(t *testing.T) {
	t.Parallel()
	doc := []byte(`{
		"dpVersion":"1.1.0",
		"title":"",
		"items":[{"source":"https://a"}],
		"signatures":[{"alg":"ed25519","kid":"did:key:z6MkhaXgBZDvotDkL5257faiztiGiC2QtKLGpbnnEGta2doK",
			"ts":"2025-01-01T00:00:00Z",
			"payload_hash":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			"role":"curator","sig":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"}]
	}`)
	_, err := dp1.ParseAndValidatePlaylist(doc)
	if err == nil {
		t.Fatal("expected schema validation error")
	}
	assertValidationErrorChain(t, err)
}

func assertValidationErrorChain(t *testing.T, err error) {
	t.Helper()
	if errors.Is(err, dp1.ErrValidation) {
		return
	}
	var coded *dp1.CodedError
	if errors.As(err, &coded) && errors.Is(coded.Err, dp1.ErrValidation) {
		return
	}
	t.Fatalf("expected validation error (or CodedError wrapping it), got %T: %v", err, err)
}

func TestParseAndValidatePlaylistWithPlaylistsExtension(t *testing.T) {
	t.Parallel()
	_, priv, _ := ed25519.GenerateKey(nil)
	kid, err := sign.Ed25519DIDKey(priv.Public().(ed25519.PublicKey))
	if err != nil {
		t.Fatal(err)
	}
	pl := playlist.Playlist{
		DPVersion: "1.1.0",
		Title:     "Ext",
		Items:     []playlist.PlaylistItem{{Source: "https://a"}},
		Summary:   "A curated feed",
		Curators: []identity.Entity{
			{Name: "Alice", Key: kid},
		},
	}
	body, _ := json.Marshal(pl)
	sig, _ := sign.SignMultiEd25519(body, priv, playlist.RoleCurator, "2025-06-01T12:00:00Z")
	pl.Signatures = []playlist.Signature{sig}
	signed, _ := json.Marshal(pl)
	out, err := dp1.ParseAndValidatePlaylistWithPlaylistsExtension(signed)
	if err != nil {
		t.Fatal(err)
	}
	if out.Summary != "A curated feed" {
		t.Fatal(out.Summary)
	}
}

// --- ParseAndValidate: playlist-group, ref manifest, channel ---

func TestParseAndValidatePlaylistGroup(t *testing.T) {
	t.Parallel()
	_, priv, _ := ed25519.GenerateKey(nil)
	g := playlistgroup.Group{
		ID:        "385f79b6-a45f-4c1c-8080-e93a192adccc",
		Title:     "Ex",
		Playlists: []string{"https://feed.example/p.json"},
		Created:   "2025-06-01T12:00:00Z",
	}
	body, _ := json.Marshal(g)
	sig, _ := sign.SignMultiEd25519(body, priv, playlist.RoleCurator, "2025-06-01T12:00:00Z")
	g.Signatures = []playlist.Signature{sig}
	signed, _ := json.Marshal(g)
	out, err := dp1.ParseAndValidatePlaylistGroup(signed)
	if err != nil {
		t.Fatal(err)
	}
	if out.Title != "Ex" {
		t.Fatal(out.Title)
	}
}

func TestParseAndValidateRefManifest(t *testing.T) {
	t.Parallel()
	m := refmanifest.Manifest{
		RefVersion: "0.1.0",
		ID:         "ref-1",
		Created:    "2025-06-01T12:00:00Z",
		Locale:     "en",
		Metadata: &refmanifest.Metadata{
			Title: "Work",
			Artists: []refmanifest.Artist{
				{Name: "A"},
			},
			Thumbnails: map[string]refmanifest.Thumbnail{
				"default": {URI: "ipfs://bafy", W: 100, H: 100},
			},
		},
	}
	data, _ := json.Marshal(m)
	out, err := dp1.ParseAndValidateRefManifest(data)
	if err != nil {
		t.Fatal(err)
	}
	if out.Metadata.Title != "Work" {
		t.Fatal(out.Metadata.Title)
	}
}

func TestParseAndValidateChannel(t *testing.T) {
	t.Parallel()
	_, priv, _ := ed25519.GenerateKey(nil)
	ch := channels.Channel{
		ID:        "385f79b6-a45f-4c1c-8080-e93a192adccc",
		Slug:      "chan",
		Title:     "Ch",
		Version:   "1.0.0",
		Created:   "2025-06-01T12:00:00Z",
		Playlists: []string{"https://feed.example/p.json"},
	}
	body, _ := json.Marshal(ch)
	sig, _ := sign.SignMultiEd25519(body, priv, playlist.RoleFeed, "2025-06-01T12:00:00Z")
	ch.Signatures = []playlist.Signature{sig}
	signed, _ := json.Marshal(ch)
	out, err := dp1.ParseAndValidateChannel(signed)
	if err != nil {
		t.Fatal(err)
	}
	if out.Title != "Ch" {
		t.Fatal(out.Title)
	}
}

// --- Decode errors (schema hooks; sequential subtests — not parallel with globals) ---

func TestParseAndValidate_decodeErrors(t *testing.T) {
	cases := []struct {
		name string
		run  func(t *testing.T)
	}{
		{"playlist", func(t *testing.T) {
			orig := dp1.PlaylistCoreSchemaValidate
			dp1.PlaylistCoreSchemaValidate = func([]byte) error { return nil }
			t.Cleanup(func() { dp1.PlaylistCoreSchemaValidate = orig })
			_, err := dp1.ParseAndValidatePlaylist([]byte(`{"dpVersion":"1.1.0","title":"x","items":[{"source":true}],"signature":"ed25519:aa"}`))
			if err == nil || !strings.Contains(err.Error(), "decode playlist") {
				t.Fatalf("got %v", err)
			}
			var coded *dp1.CodedError
			if !errors.As(err, &coded) || coded.Code != dp1.CodePlaylistInvalid {
				t.Fatalf("expected *CodedError with CodePlaylistInvalid, got %v", err)
			}
		}},
		{"playlist_ext", func(t *testing.T) {
			orig := dp1.PlaylistWithPlaylistsExtensionSchemaValidate
			dp1.PlaylistWithPlaylistsExtensionSchemaValidate = func([]byte) error { return nil }
			t.Cleanup(func() { dp1.PlaylistWithPlaylistsExtensionSchemaValidate = orig })
			_, err := dp1.ParseAndValidatePlaylistWithPlaylistsExtension([]byte(`{"dpVersion":"1.1.0","title":"x","items":[{"source":true}],"signatures":[]}`))
			if err == nil || !strings.Contains(err.Error(), "decode playlist") {
				t.Fatalf("got %v", err)
			}
			var coded *dp1.CodedError
			if !errors.As(err, &coded) || coded.Code != dp1.CodePlaylistInvalid {
				t.Fatalf("expected CodePlaylistInvalid, got %v", err)
			}
		}},
		{"group", func(t *testing.T) {
			orig := dp1.PlaylistGroupSchemaValidate
			dp1.PlaylistGroupSchemaValidate = func([]byte) error { return nil }
			t.Cleanup(func() { dp1.PlaylistGroupSchemaValidate = orig })
			_, err := dp1.ParseAndValidatePlaylistGroup([]byte(`{"id":"385f79b6-a45f-4c1c-8080-e93a192adccc","title":1,"playlists":[],"created":"2025-01-01T00:00:00Z","signature":"ed25519:aa"}`))
			if err == nil || !strings.Contains(err.Error(), "decode playlist-group") {
				t.Fatalf("got %v", err)
			}
			var coded *dp1.CodedError
			if !errors.As(err, &coded) || coded.Code != dp1.CodePlaylistGroupInvalid {
				t.Fatalf("expected CodePlaylistGroupInvalid, got %v", err)
			}
		}},
		{"ref", func(t *testing.T) {
			orig := dp1.RefManifestSchemaValidate
			dp1.RefManifestSchemaValidate = func([]byte) error { return nil }
			t.Cleanup(func() { dp1.RefManifestSchemaValidate = orig })
			_, err := dp1.ParseAndValidateRefManifest([]byte(`{"refVersion":"0.1.0","id":1,"created":"2025-01-01T00:00:00Z","locale":"en"}`))
			if err == nil || !strings.Contains(err.Error(), "decode ref manifest") {
				t.Fatalf("got %v", err)
			}
			var coded *dp1.CodedError
			if !errors.As(err, &coded) || coded.Code != dp1.CodeRefManifestInvalid {
				t.Fatalf("expected CodeRefManifestInvalid, got %v", err)
			}
		}},
		{"channel", func(t *testing.T) {
			orig := dp1.ChannelExtensionSchemaValidate
			dp1.ChannelExtensionSchemaValidate = func([]byte) error { return nil }
			t.Cleanup(func() { dp1.ChannelExtensionSchemaValidate = orig })
			_, err := dp1.ParseAndValidateChannel([]byte(`{"id":"385f79b6-a45f-4c1c-8080-e93a192adccc","slug":"s","title":1,"version":"1.0.0","created":"2025-01-01T00:00:00Z","playlists":[],"signature":"ed25519:aa"}`))
			if err == nil || !strings.Contains(err.Error(), "decode channel") {
				t.Fatalf("got %v", err)
			}
			var coded *dp1.CodedError
			if !errors.As(err, &coded) || coded.Code != dp1.CodeChannelInvalid {
				t.Fatalf("expected CodeChannelInvalid, got %v", err)
			}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.run(t)
		})
	}
}

// --- Version ---

func TestParseDPVersion(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		t.Parallel()
		v, err := dp1.ParseDPVersion("1.1.0")
		if err != nil {
			t.Fatal(err)
		}
		if v.String() != "1.1.0" {
			t.Fatal(v.String())
		}
	})
	t.Run("invalid", func(t *testing.T) {
		t.Parallel()
		_, err := dp1.ParseDPVersion("not-semver")
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestWarnMajorMismatch(t *testing.T) {
	t.Run("mismatch", func(t *testing.T) {
		t.Parallel()
		v, _ := dp1.ParseDPVersion("2.0.0")
		if err := dp1.WarnMajorMismatch(v, 1); err == nil {
			t.Fatal("expected mismatch")
		}
		if err := dp1.WarnMajorMismatch(v, 2); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("nil_version", func(t *testing.T) {
		t.Parallel()
		if err := dp1.WarnMajorMismatch(nil, 1); err != nil {
			t.Fatal(err)
		}
	})
}

// --- Coded errors ---

func TestCodeFromValidationWrappers(t *testing.T) {
	cases := []struct {
		name string
		wrap func(error) error
		want dp1.ErrorCode
	}{
		{"playlist", dp1.CodeFromPlaylistValidation, dp1.CodePlaylistInvalid},
		{"playlist_group", dp1.CodeFromPlaylistGroupValidation, dp1.CodePlaylistGroupInvalid},
		{"ref_manifest", dp1.CodeFromRefManifestValidation, dp1.CodeRefManifestInvalid},
		{"channel", dp1.CodeFromChannelValidation, dp1.CodeChannelInvalid},
	}
	for _, tc := range cases {
		t.Run(tc.name+"_wraps_validation", func(t *testing.T) {
			t.Parallel()
			err := tc.wrap(dp1.ErrValidation)
			if err == nil {
				t.Fatal("expected error")
			}
			var coded *dp1.CodedError
			if !errors.As(err, &coded) || coded.Code != tc.want {
				t.Fatalf("got %v", err)
			}
		})
		t.Run(tc.name+"_passthrough_other", func(t *testing.T) {
			t.Parallel()
			e := errors.New("other")
			if got := tc.wrap(e); !errors.Is(got, e) {
				t.Fatal("expected same error")
			}
		})
	}
}

func TestCodedError_and_WithCode(t *testing.T) {
	t.Run("unwrap", func(t *testing.T) {
		t.Parallel()
		inner := errors.New("inner")
		err := dp1.WithCode(dp1.CodePlaylistInvalid, inner)
		var ce *dp1.CodedError
		if !errors.As(err, &ce) {
			t.Fatal("expected CodedError")
		}
		if !errors.Is(err, inner) {
			t.Fatal("unwrap")
		}
		if ce.Error() == "" {
			t.Fatal("empty")
		}
	})
	t.Run("nil", func(t *testing.T) {
		t.Parallel()
		if dp1.WithCode(dp1.CodePlaylistInvalid, nil) != nil {
			t.Fatal("expected nil")
		}
	})
}
