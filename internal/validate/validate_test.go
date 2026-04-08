package validate

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
)

func TestCompilerSingleton_Idempotent(t *testing.T) {
	t.Parallel()
	c1, err := compilerSingleton()
	if err != nil {
		t.Fatal(err)
	}
	c2, err := compilerSingleton()
	if err != nil {
		t.Fatal(err)
	}
	if c1 != c2 {
		t.Fatal("expected same compiler instance")
	}
}

// playlistSigBlock is a syntactically valid multi-sig object for core playlist / overlay tests.
const playlistSigBlock = `"signatures":[{"alg":"ed25519","kid":"did:key:z6MkhaXgBZDvotDkL5257faiztiGiC2QtKLGpbnnEGta2doK",
			"ts":"2025-01-01T00:00:00Z",
			"payload_hash":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			"role":"curator","sig":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"}]`

func TestValidateAgainst_invalidJSON(t *testing.T) {
	t.Parallel()
	err := validateAgainst(playlistSchemaURL, []byte(`{"dpVersion":`))
	assertErrValidation(t, err)
	if !strings.Contains(err.Error(), "json:") {
		t.Fatalf("expected json decode detail, got: %v", err)
	}
}

func TestValidateAgainst_unknownSchemaURL(t *testing.T) {
	t.Parallel()
	err := validateAgainst("https://dp1.feralfile.com/schemas/v9.9.9/does-not-exist.json", []byte(`{}`))
	if err == nil {
		t.Fatal("expected compile error")
	}
	if errors.Is(err, ErrValidation) {
		t.Fatal("compile failure must not wrap ErrValidation")
	}
	if !strings.Contains(err.Error(), "compile schema") {
		t.Fatalf("expected compile schema message, got: %v", err)
	}
}

func TestPlaylist_MissingSignature(t *testing.T) {
	t.Parallel()
	err := Playlist([]byte(`{"dpVersion":"1.1.0","title":"x","items":[{"source":"https://a"}]}`))
	assertErrValidation(t, err)
}

func TestPlaylist_validationFailures(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		doc  string
	}{
		{"missing_dpVersion", fmt.Sprintf(`{"title":"x","items":[{"source":"https://a"}],%s}`, playlistSigBlock)},
		{"missing_title", fmt.Sprintf(`{"dpVersion":"1.1.0","items":[{"source":"https://a"}],%s}`, playlistSigBlock)},
		{"missing_items", fmt.Sprintf(`{"dpVersion":"1.1.0","title":"x",%s}`, playlistSigBlock)},
		{"empty_title", fmt.Sprintf(`{"dpVersion":"1.1.0","title":"","items":[{"source":"https://a"}],%s}`, playlistSigBlock)},
		{"dpVersion_bad_semver", fmt.Sprintf(`{"dpVersion":"1.0","title":"x","items":[{"source":"https://a"}],%s}`, playlistSigBlock)},
		{"items_empty", fmt.Sprintf(`{"dpVersion":"1.1.0","title":"x","items":[],%s}`, playlistSigBlock)},
		{"item_missing_source", fmt.Sprintf(`{"dpVersion":"1.1.0","title":"x","items":[{}],%s}`, playlistSigBlock)},
		{"item_source_not_uri", fmt.Sprintf(`{"dpVersion":"1.1.0","title":"x","items":[{"source":"not a uri"}],%s}`, playlistSigBlock)},
		{"id_bad_uuid", fmt.Sprintf(`{"dpVersion":"1.1.0","title":"x","id":"nope","items":[{"source":"https://a"}],%s}`, playlistSigBlock)},
		{"legacy_signature_bad_pattern", `{"dpVersion":"1.1.0","title":"x","items":[{"source":"https://a"}],"signature":"wrong"}`},
		{"sig_alg_invalid", `{"dpVersion":"1.1.0","title":"x","items":[{"source":"https://a"}],
			"signatures":[{"alg":"rsa999","kid":"did:key:z6MkhaXgBZDvotDkL5257faiztiGiC2QtKLGpbnnEGta2doK",
			"ts":"2025-01-01T00:00:00Z","payload_hash":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			"role":"curator","sig":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"}]}`},
		{"sig_kid_not_did", `{"dpVersion":"1.1.0","title":"x","items":[{"source":"https://a"}],
			"signatures":[{"alg":"ed25519","kid":"not-a-did",
			"ts":"2025-01-01T00:00:00Z","payload_hash":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			"role":"curator","sig":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"}]}`},
		{"sig_payload_hash_uppercase_hex", `{"dpVersion":"1.1.0","title":"x","items":[{"source":"https://a"}],
			"signatures":[{"alg":"ed25519","kid":"did:key:z6MkhaXgBZDvotDkL5257faiztiGiC2QtKLGpbnnEGta2doK",
			"ts":"2025-01-01T00:00:00Z","payload_hash":"sha256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
			"role":"curator","sig":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"}]}`},
		{"sig_role_invalid", `{"dpVersion":"1.1.0","title":"x","items":[{"source":"https://a"}],
			"signatures":[{"alg":"ed25519","kid":"did:key:z6MkhaXgBZDvotDkL5257faiztiGiC2QtKLGpbnnEGta2doK",
			"ts":"2025-01-01T00:00:00Z","payload_hash":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			"role":"owner","sig":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"}]}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assertErrValidation(t, Playlist([]byte(tc.doc)))
		})
	}
}

const groupSigBlock = `"signatures":[{"alg":"ed25519","kid":"did:key:z6MkhaXgBZDvotDkL5257faiztiGiC2QtKLGpbnnEGta2doK",
			"ts":"2025-01-01T00:00:00Z",
			"payload_hash":"sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
			"role":"feed","sig":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"}]`

func TestPlaylistGroup_validationFailures(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		doc  string
	}{
		{"missing_id", `{"title":"g","playlists":["https://p"],"created":"2025-01-01T00:00:00Z",` + groupSigBlock + `}`},
		{"id_not_uuid", `{"id":"not-uuid","title":"g","playlists":["https://p"],"created":"2025-01-01T00:00:00Z",` + groupSigBlock + `}`},
		{"playlists_empty", `{"id":"385f79b6-a45f-4c1c-8080-e93a192adccc","title":"g","playlists":[],"created":"2025-01-01T00:00:00Z",` + groupSigBlock + `}`},
		{"playlist_entry_not_uri", `{"id":"385f79b6-a45f-4c1c-8080-e93a192adccc","title":"g","playlists":["%%%"],"created":"2025-01-01T00:00:00Z",` + groupSigBlock + `}`},
		{"created_bad_datetime", `{"id":"385f79b6-a45f-4c1c-8080-e93a192adccc","title":"g","playlists":["https://p"],"created":"yesterday",` + groupSigBlock + `}`},
		{"no_signature", `{"id":"385f79b6-a45f-4c1c-8080-e93a192adccc","title":"g","playlists":["https://p"],"created":"2025-01-01T00:00:00Z"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assertErrValidation(t, PlaylistGroup([]byte(tc.doc)))
		})
	}
}

func TestRefManifest_validationFailures(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		doc  string
	}{
		{"missing_refVersion", `{"id":"r","created":"2025-01-01T00:00:00Z","locale":"en"}`},
		{"refVersion_bad_pattern", `{"refVersion":"0.1","id":"r","created":"2025-01-01T00:00:00Z","locale":"en"}`},
		{"id_empty", `{"refVersion":"0.1.0","id":"","created":"2025-01-01T00:00:00Z","locale":"en"}`},
		{"created_bad_datetime", `{"refVersion":"0.1.0","id":"r","created":"not-rfc3339","locale":"en"}`},
		{"locale_bad", `{"refVersion":"0.1.0","id":"r","created":"2025-01-01T00:00:00Z","locale":"english"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assertErrValidation(t, RefManifest([]byte(tc.doc)))
		})
	}
}

const channelSigBlock = `"signatures":[{"alg":"ed25519","kid":"did:key:z6MkhaXgBZDvotDkL5257faiztiGiC2QtKLGpbnnEGta2doK",
			"ts":"2025-01-01T00:00:00Z",
			"payload_hash":"sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
			"role":"curator","sig":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"}]`

func TestChannelsExtension_validationFailures(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		doc  string
	}{
		{"missing_slug", `{"id":"385f79b6-a45f-4c1c-8080-e93a192adccc","title":"c","version":"1.0.0","created":"2025-01-01T00:00:00Z","playlists":["https://p"],` + channelSigBlock + `}`},
		{"slug_bad_pattern", `{"id":"385f79b6-a45f-4c1c-8080-e93a192adccc","slug":"Bad_Slug","title":"c","version":"1.0.0","created":"2025-01-01T00:00:00Z","playlists":["https://p"],` + channelSigBlock + `}`},
		{"version_bad_semver", `{"id":"385f79b6-a45f-4c1c-8080-e93a192adccc","slug":"s","title":"c","version":"1.0","created":"2025-01-01T00:00:00Z","playlists":["https://p"],` + channelSigBlock + `}`},
		{"playlists_empty", `{"id":"385f79b6-a45f-4c1c-8080-e93a192adccc","slug":"s","title":"c","version":"1.0.0","created":"2025-01-01T00:00:00Z","playlists":[],` + channelSigBlock + `}`},
		{"id_not_uuid", `{"id":"x","slug":"s","title":"c","version":"1.0.0","created":"2025-01-01T00:00:00Z","playlists":["https://p"],` + channelSigBlock + `}`},
		{"no_signature", `{"id":"385f79b6-a45f-4c1c-8080-e93a192adccc","slug":"s","title":"c","version":"1.0.0","created":"2025-01-01T00:00:00Z","playlists":["https://p"]}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assertErrValidation(t, ChannelsExtension([]byte(tc.doc)))
		})
	}
}

func TestPlaylistsExtensionFragment_validationFailures(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		doc  string
	}{
		{"summary_empty", `{"summary":""}`},
		{"coverImage_not_uri", `{"coverImage":"not a uri"}`},
		{"curator_missing_key", `{"curators":[{"name":"A"}]}`},
		{"dynamicQuery_incomplete", `{"dynamicQuery":{"profile":"https-json-v1"}}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assertErrValidation(t, PlaylistsExtensionFragment([]byte(tc.doc)))
		})
	}
}

func TestPlaylistWithPlaylistsExtension_validationFailures(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		doc  string
	}{
		{"core_missing_title", fmt.Sprintf(`{"dpVersion":"1.1.0","items":[{"source":"https://a"}],%s}`, playlistSigBlock)},
		{"extension_summary_empty", fmt.Sprintf(`{"dpVersion":"1.1.0","title":"x","items":[{"source":"https://a"}],"summary":"",%s}`, playlistSigBlock)},
		{"extension_cover_bad_uri", fmt.Sprintf(`{"dpVersion":"1.1.0","title":"x","items":[{"source":"https://a"}],"coverImage":"not a uri",%s}`, playlistSigBlock)},
		{"items_empty_no_dynamic_query", fmt.Sprintf(`{"dpVersion":"1.1.0","title":"x","items":[],%s}`, playlistSigBlock)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assertErrValidation(t, PlaylistWithPlaylistsExtension([]byte(tc.doc)))
		})
	}
}

func TestValidators_minimalValid(t *testing.T) {
	t.Parallel()
	playlistCore := []byte(`{
		"dpVersion":"1.1.0",
		"title":"x",
		"items":[{"source":"https://a"}],
		"signatures":[{"alg":"ed25519","kid":"did:key:z6MkhaXgBZDvotDkL5257faiztiGiC2QtKLGpbnnEGta2doK",
			"ts":"2025-01-01T00:00:00Z",
			"payload_hash":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			"role":"curator","sig":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"}]
	}`)
	if err := Playlist(playlistCore); err != nil {
		t.Fatal(err)
	}
	group := []byte(`{
		"id":"385f79b6-a45f-4c1c-8080-e93a192adccc",
		"title":"g",
		"playlists":["https://p"],
		"created":"2025-01-01T00:00:00Z",
		"signatures":[{"alg":"ed25519","kid":"did:key:z6MkhaXgBZDvotDkL5257faiztiGiC2QtKLGpbnnEGta2doK",
			"ts":"2025-01-01T00:00:00Z",
			"payload_hash":"sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
			"role":"feed","sig":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"}]
	}`)
	if err := PlaylistGroup(group); err != nil {
		t.Fatal(err)
	}
	ref := []byte(`{
		"refVersion":"0.1.0",
		"id":"r",
		"created":"2025-01-01T00:00:00Z",
		"locale":"en"
	}`)
	if err := RefManifest(ref); err != nil {
		t.Fatal(err)
	}
	ch := []byte(`{
		"id":"385f79b6-a45f-4c1c-8080-e93a192adccc",
		"slug":"s",
		"title":"c",
		"version":"1.0.0",
		"created":"2025-01-01T00:00:00Z",
		"playlists":["https://p"],
		"signatures":[{"alg":"ed25519","kid":"did:key:z6MkhaXgBZDvotDkL5257faiztiGiC2QtKLGpbnnEGta2doK",
			"ts":"2025-01-01T00:00:00Z",
			"payload_hash":"sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
			"role":"curator","sig":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"}]
	}`)
	if err := ChannelsExtension(ch); err != nil {
		t.Fatal(err)
	}
	extOnly := []byte(`{"summary":"x"}`)
	if err := PlaylistsExtensionFragment(extOnly); err != nil {
		t.Fatal(err)
	}
	overlay := []byte(`{
		"dpVersion":"1.1.0",
		"title":"x",
		"items":[{"source":"https://a"}],
		"summary":"ext",
		"signatures":[{"alg":"ed25519","kid":"did:key:z6MkhaXgBZDvotDkL5257faiztiGiC2QtKLGpbnnEGta2doK",
			"ts":"2025-01-01T00:00:00Z",
			"payload_hash":"sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd",
			"role":"curator","sig":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"}]
	}`)
	if err := PlaylistWithPlaylistsExtension(overlay); err != nil {
		t.Fatal(err)
	}
}

func TestValidate_concurrentPlaylist(t *testing.T) {
	doc := []byte(`{
		"dpVersion":"1.1.0",
		"title":"x",
		"items":[{"source":"https://a"}],
		"signatures":[{"alg":"ed25519","kid":"did:key:z6MkhaXgBZDvotDkL5257faiztiGiC2QtKLGpbnnEGta2doK",
			"ts":"2025-01-01T00:00:00Z",
			"payload_hash":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			"role":"curator","sig":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"}]
	}`)
	var wg sync.WaitGroup
	for range 32 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := Playlist(doc); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
}

func TestPlaylistItem_OK_and_invalid(t *testing.T) {
	t.Parallel()
	if err := PlaylistItem([]byte(`{"source":"https://example.com/a"}`)); err != nil {
		t.Fatal(err)
	}
	assertErrValidation(t, PlaylistItem([]byte(`{}`)))
}

func assertErrValidation(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("expected errors.Is(err, ErrValidation), got: %v", err)
	}
}
