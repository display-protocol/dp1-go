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
	"github.com/display-protocol/dp1-go/extension/playlists"
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
	dur := 15.0
	pl := playlist.Playlist{
		DPVersion: "1.1.0",
		Title:     "Ext",
		Note:      &playlists.Note{Text: "Welcome", Duration: &dur},
		Items: []playlist.PlaylistItem{{
			Source: "https://a",
			Note:   &playlists.Note{Text: "Track intro"},
		}},
		Summary: "A curated feed",
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
	if out.Note == nil || out.Note.Text != "Welcome" || out.Note.Duration == nil || *out.Note.Duration != 15 {
		t.Fatalf("note: %+v", out.Note)
	}
	if len(out.Items) != 1 || out.Items[0].Note == nil || out.Items[0].Note.Text != "Track intro" {
		t.Fatalf("item: %+v", out.Items)
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
				"default": {URI: "ipfs://bafy", W: intPtr(100), H: intPtr(100)},
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

func intPtr(i int) *int { return &i }

// dummySignatureJSON is schema-valid but cryptographically meaningless. ParseAndValidate*
// only runs JSON Schema, so tests that target schema shape use it instead of signing —
// keeping the assertion on the field under test rather than on signature plumbing.
const dummySignatureJSON = `{"alg":"ed25519","kid":"did:key:z6MkhaXgBZDvotDkL5257faiztiGiC2QtKLGpbnnEGta2doK",
	"ts":"2025-01-01T00:00:00Z",
	"payload_hash":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	"role":"curator","sig":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"}`

func dummySignature(t *testing.T) playlist.Signature {
	t.Helper()
	var s playlist.Signature
	if err := json.Unmarshal([]byte(dummySignatureJSON), &s); err != nil {
		t.Fatal(err)
	}
	return s
}

// Thumbnail.w / Thumbnail.h were dropped from the ref-manifest required list
// (core changelog 2026-08-12): a bare thumbnail URL must validate on its own.
func TestParseAndValidateRefManifest_thumbnailWithoutDimensions(t *testing.T) {
	t.Parallel()
	m := refmanifest.Manifest{
		RefVersion: "0.1.0",
		ID:         "ref-1",
		Created:    "2025-06-01T12:00:00Z",
		Locale:     "en",
		Metadata: &refmanifest.Metadata{
			Thumbnails: map[string]refmanifest.Thumbnail{
				"default": {URI: "https://example.com/thumb.png"},
			},
		},
	}
	data, _ := json.Marshal(m)
	if strings.Contains(string(data), `"w"`) || strings.Contains(string(data), `"h"`) {
		t.Fatalf("absent dimensions must not marshal: %s", data)
	}
	out, err := dp1.ParseAndValidateRefManifest(data)
	if err != nil {
		t.Fatal(err)
	}
	th := out.Metadata.Thumbnails["default"]
	if th.W != nil || th.H != nil {
		t.Fatalf("expected nil dimensions, got w=%v h=%v", th.W, th.H)
	}
}

func TestParseAndValidateRefManifest_rejectsZeroThumbnailWidth(t *testing.T) {
	t.Parallel()
	// w stays constrained when present (minimum 1); only the requirement was relaxed.
	doc := []byte(`{"refVersion":"0.1.0","id":"ref-1","created":"2025-06-01T12:00:00Z","locale":"en",
		"metadata":{"thumbnails":{"default":{"uri":"https://example.com/t.png","w":0,"h":10}}}}`)
	if _, err := dp1.ParseAndValidateRefManifest(doc); err == nil {
		t.Fatal("expected validation error for w=0")
	}
}

// Playlists extension §3.6: an item may carry a full ref manifest inline, validated by the
// unmodified ref-manifest schema.
func TestParseAndValidatePlaylistWithPlaylistsExtension_inlineManifest(t *testing.T) {
	t.Parallel()
	pl := playlist.Playlist{
		DPVersion: "1.1.0",
		Title:     "Inline",
		Items: []playlist.PlaylistItem{{
			Source: "https://example.com/work.html",
			InlineManifest: &refmanifest.Manifest{
				RefVersion: "0.1.0",
				ID:         "ref-9d26ecb3",
				Created:    "2026-07-28T00:00:00Z",
				Locale:     "en",
				Metadata: &refmanifest.Metadata{
					Title:   "Pre-Process",
					Artists: []refmanifest.Artist{{Name: "Casey Reas"}},
					Thumbnails: map[string]refmanifest.Thumbnail{
						"default": {URI: "https://example.com/thumb.png"},
					},
				},
			},
		}},
		Signatures: []playlist.Signature{dummySignature(t)},
	}
	data, _ := json.Marshal(pl)
	out, err := dp1.ParseAndValidatePlaylistWithPlaylistsExtension(data)
	if err != nil {
		t.Fatal(err)
	}
	im := out.Items[0].InlineManifest
	if im == nil || im.ID != "ref-9d26ecb3" || im.Metadata.Title != "Pre-Process" {
		t.Fatalf("inlineManifest: %+v", im)
	}
}

func TestParseAndValidatePlaylistWithPlaylistsExtension_rejectsMalformedInlineManifest(t *testing.T) {
	t.Parallel()
	// Missing the required `locale` envelope field: invalid exactly as a fetched manifest would be.
	doc := []byte(`{"dpVersion":"1.1.0","title":"Inline","items":[{"source":"https://a",
		"inlineManifest":{"refVersion":"0.1.0","id":"ref-1","created":"2026-07-28T00:00:00Z"}}],
		"signatures":[` + dummySignatureJSON + `]}`)
	_, err := dp1.ParseAndValidatePlaylistWithPlaylistsExtension(doc)
	if err == nil {
		t.Fatal("expected validation error for malformed inlineManifest")
	}
	assertValidationErrorChain(t, err)
}

// Core DP-1 tolerates unknown fields, so inlineManifest must pass the core-only path untouched.
func TestParseAndValidatePlaylist_coreIgnoresInlineManifest(t *testing.T) {
	t.Parallel()
	doc := []byte(`{"dpVersion":"1.1.0","title":"Inline","items":[{"source":"https://a",
		"inlineManifest":{"refVersion":"0.1.0","id":"ref-1","created":"2026-07-28T00:00:00Z"}}],
		"signatures":[` + dummySignatureJSON + `]}`)
	if _, err := dp1.ParseAndValidatePlaylist(doc); err != nil {
		t.Fatal(err)
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

// §3.6 puts the inline manifest's bytes inside the signed payload, so signing and verifying
// must run on the raw document. The typed structs are not byte-faithful: omitempty drops
// present-but-empty fields, and the §3.6 example manifest has one (an artist with "id": "").
// This locks both halves — raw bytes verify, a re-marshaled struct does not — so the hazard
// is a tested property rather than something a caller discovers as a bogus sigInvalid.
func TestInlineManifest_signOverRawBytesNotStructRoundTrip(t *testing.T) {
	t.Parallel()
	_, priv, _ := ed25519.GenerateKey(nil)
	body := []byte(`{"dpVersion":"1.1.0","title":"Inline","items":[{"source":"https://example.com/a",` +
		`"inlineManifest":{"refVersion":"0.1.0","id":"ref-9d26ecb3","created":"2026-07-28T00:00:00Z","locale":"en",` +
		`"metadata":{"title":"Pre-Process","artists":[{"name":"Casey Reas","id":""}]}}}]}`)
	sig, err := sign.SignMultiEd25519(body, priv, playlist.RoleCurator, "2026-07-28T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	sigJSON, _ := json.Marshal([]playlist.Signature{sig})
	signed := append(body[:len(body)-1], []byte(`,"signatures":`+string(sigJSON)+`}`)...)

	if _, err := dp1.ParseAndValidatePlaylistWithPlaylistsExtension(signed); err != nil {
		t.Fatal(err)
	}
	ok, failed, err := sign.VerifyPlaylistSignatures(signed)
	if err != nil || !ok {
		t.Fatalf("raw bytes must verify: ok=%v failed=%+v err=%v", ok, failed, err)
	}

	// Same document, re-encoded from the decoded structs: the empty artist id is gone, so the
	// JCS payload differs and the signature no longer matches.
	decoded, err := dp1.ParseAndValidatePlaylistWithPlaylistsExtension(signed)
	if err != nil {
		t.Fatal(err)
	}
	remarshaled, err := json.Marshal(decoded)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(remarshaled), `"id":""`) {
		t.Fatal("expected omitempty to drop the empty artist id; update this test if Artist.ID changed")
	}
	ok, _, err = sign.VerifyPlaylistSignatures(remarshaled)
	if err == nil && ok {
		t.Fatal("re-marshaled document verified: struct round trip is now byte-faithful, so the warning on PlaylistItem.InlineManifest is stale")
	}
}
