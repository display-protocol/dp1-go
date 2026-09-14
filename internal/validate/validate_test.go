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
		// Artist profile (refVersion 1.1.0). Each case violates exactly one
		// constraint so that it stops passing only when that constraint drops
		// out of the schema.
		{"artist_address_empty", refManifestWithArtist(`{"name":"A","addresses":[""]}`)},
		{"artist_biography_missing_text", refManifestWithArtist(`{"name":"A","biographies":[{"source":"DAM"}]}`)},
		{"artist_biography_text_empty", refManifestWithArtist(`{"name":"A","biographies":[{"text":""}]}`)},
		{"artist_link_missing_url", refManifestWithArtist(`{"name":"A","links":[{"type":"twitter"}]}`)},
		{"artist_link_bare_handle", refManifestWithArtist(`{"name":"A","links":[{"type":"twitter","url":"@REAS"}]}`)},
		{"artist_link_unknown_type", refManifestWithArtist(`{"name":"A","links":[{"type":"mastodon","url":"https://example.social/@a"}]}`)},
		{"artist_avatar_missing_uri", refManifestWithArtist(`{"name":"A","avatar":{"w":512,"h":512}}`)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assertErrValidation(t, RefManifest([]byte(tc.doc)))
		})
	}
}

// refManifestWithArtist wraps one artist object in an otherwise valid 1.1.0
// manifest, so an artist-profile test case names only the field it probes.
func refManifestWithArtist(artist string) string {
	return `{"refVersion":"1.1.0","id":"r","created":"2025-01-01T00:00:00Z","locale":"en","metadata":{"artists":[` + artist + `]}}`
}

func TestRefManifest_artistProfileAccepted(t *testing.T) {
	t.Parallel()
	// The full 1.1.0 profile as a producer should emit it: no deprecated url,
	// full URLs in links, w/h present on the avatar.
	doc := refManifestWithArtist(`{
		"name":"Casey Reas","id":"58",
		"addresses":["0x457ee5f723c7606c12a7264b52e285906f91eea6","tz1LBwyJMRkH4tcG19KwYzAW7fLYbjFmWdWy"],
		"avatar":{"uri":"https://example.com/avatar.jpg","w":512,"h":512},
		"biographies":[{"text":"Software artist.","source":"DAM","sourceUrl":"https://dam.org/reas"},{"text":"Co-founder of Processing."}],
		"links":[{"type":"website","url":"https://reas.com"},{"type":"other","url":"https://example.social/@reas"}]
	}`)
	if err := RefManifest([]byte(doc)); err != nil {
		t.Fatal(err)
	}
	// A 1.0.0 artist — name/id/url only — stays valid: the bump is additive
	// and url, though deprecated, is still accepted.
	legacy := refManifestWithArtist(`{"name":"A","id":"","url":"https://a.example"}`)
	if err := RefManifest([]byte(legacy)); err != nil {
		t.Fatal(err)
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
		{"sig_role_invalid", `{"id":"385f79b6-a45f-4c1c-8080-e93a192adccc","slug":"s","title":"c","version":"1.0.0","created":"2025-01-01T00:00:00Z","playlists":["https://p"],
			"signatures":[{"alg":"ed25519","kid":"did:key:z6MkhaXgBZDvotDkL5257faiztiGiC2QtKLGpbnnEGta2doK",
			"ts":"2025-01-01T00:00:00Z","payload_hash":"sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
			"role":"owner","sig":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"}]}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assertErrValidation(t, ChannelsExtension([]byte(tc.doc)))
		})
	}
}

func TestChannelsExtension_publisherRole(t *testing.T) {
	t.Parallel()
	doc := []byte(`{
		"id":"385f79b6-a45f-4c1c-8080-e93a192adccc",
		"slug":"s",
		"title":"c",
		"version":"1.0.0",
		"created":"2025-01-01T00:00:00Z",
		"playlists":["https://p"],
		"signatures":[{"alg":"ed25519","kid":"did:key:z6MkhaXgBZDvotDkL5257faiztiGiC2QtKLGpbnnEGta2doK",
			"ts":"2025-01-01T00:00:00Z",
			"payload_hash":"sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
			"role":"publisher","sig":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"}]
	}`)
	if err := ChannelsExtension(doc); err != nil {
		t.Fatal(err)
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
		{"note_text_empty", `{"note":{"text":""}}`},
		{"note_duration_not_positive", `{"note":{"text":"x","duration":0}}`},
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
		{"note_text_empty", fmt.Sprintf(`{"dpVersion":"1.1.0","title":"x","note":{"text":""},"items":[{"source":"https://a"}],%s}`, playlistSigBlock)},
		{"item_note_duration_zero", fmt.Sprintf(`{"dpVersion":"1.1.0","title":"x","items":[{"source":"https://a","note":{"text":"x","duration":0}}],%s}`, playlistSigBlock)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assertErrValidation(t, PlaylistWithPlaylistsExtension([]byte(tc.doc)))
		})
	}
}

func TestPlaylistWithPlaylistsExtension_withNotes(t *testing.T) {
	t.Parallel()
	doc := fmt.Sprintf(`{
		"dpVersion":"1.1.0",
		"title":"x",
		"note":{"text":"Before the show","duration":12},
		"items":[{"source":"https://a","note":{"text":"Before first work"}}],
		%s}`, playlistSigBlock)
	if err := PlaylistWithPlaylistsExtension([]byte(doc)); err != nil {
		t.Fatal(err)
	}
}

func TestPlaylistWithPlaylistsExtension_emptyItemsWithDynamicQuery(t *testing.T) {
	t.Parallel()
	doc := fmt.Sprintf(`{
		"dpVersion":"1.1.0",
		"title":"x",
		"items":[],
		"dynamicQuery":{
			"profile":"graphql-v1",
			"endpoint":"https://example.com/graphql",
			"responseMapping":{"itemsPath":"data.items","itemSchema":"dp1/1.1"}
		},
		%s}`, playlistSigBlock)
	if err := PlaylistWithPlaylistsExtension([]byte(doc)); err != nil {
		t.Fatal(err)
	}
}

func TestPlaylistWithPlaylistsExtension_displayAt(t *testing.T) {
	t.Parallel()
	doc := fmt.Sprintf(`{
		"dpVersion":"1.1.0",
		"title":"Daily",
		"items":[
			{"source":"https://a.com/intro"},
			{"source":"https://a.com/day1","displayAt":"2026-07-21T00:00:00"},
			{"source":"https://a.com/day2","displayAt":"2026-07-22T00:00:00"},
			{"source":"https://a.com/day3","displayAt":"2026-07-23T00:00:00Z"},
			{"source":"https://a.com/day4","displayAt":"2026-07-24T09:00:00+07:00"}
		],
		%s}`, playlistSigBlock)
	if err := PlaylistWithPlaylistsExtension([]byte(doc)); err != nil {
		t.Fatal(err)
	}
}

func TestPlaylistWithPlaylistsExtension_displayAtValidationFailures(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		displayAt string
	}{
		{"date_only", "2026-07-21"},
		{"compact_offset_no_colon", "2026-07-21T00:00:00+0700"},
		{"invalid_month", "2026-13-01T00:00:00"},
		{"invalid_day", "2026-07-32T00:00:00"},
		{"invalid_hour", "2026-07-21T25:00:00"},
		{"wrong_separator", "2026/07/21"},
		{"missing_seconds", "2026-07-21T00:00"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			doc := fmt.Sprintf(`{
				"dpVersion":"1.1.0",
				"title":"x",
				"items":[{"source":"https://a","displayAt":%q}],
				%s}`, tc.displayAt, playlistSigBlock)
			assertErrValidation(t, PlaylistWithPlaylistsExtension([]byte(doc)))
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
	noteOnly := []byte(`{"note":{"text":"hello"}}`)
	if err := PlaylistsExtensionFragment(noteOnly); err != nil {
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

func TestPlaylistItemWithPlaylistsExtension_OK_and_displayAt(t *testing.T) {
	t.Parallel()
	if err := PlaylistItemWithPlaylistsExtension([]byte(`{"source":"https://example.com/a"}`)); err != nil {
		t.Fatal(err)
	}
	if err := PlaylistItemWithPlaylistsExtension([]byte(`{"source":"https://example.com/a","displayAt":"2026-07-21T00:00:00Z"}`)); err != nil {
		t.Fatal(err)
	}
	if err := PlaylistItemWithPlaylistsExtension([]byte(`{"source":"https://example.com/a","displayAt":"2026-07-21T00:00:00"}`)); err != nil {
		t.Fatal(err)
	}
	assertErrValidation(t, PlaylistItemWithPlaylistsExtension([]byte(`{}`)))
	assertErrValidation(t, PlaylistItemWithPlaylistsExtension([]byte(`{"source":"https://example.com/a","displayAt":null}`)))
	assertErrValidation(t, PlaylistItemWithPlaylistsExtension([]byte(`{"source":"https://example.com/a","displayAt":""}`)))
	assertErrValidation(t, PlaylistItemWithPlaylistsExtension([]byte(`{"source":"https://example.com/a","displayAt":"not-a-date"}`)))
	assertErrValidation(t, PlaylistItemWithPlaylistsExtension([]byte(`{"source":"https://example.com/a","displayAt":"2026-07-21"}`)))
	assertErrValidation(t, PlaylistItemWithPlaylistsExtension([]byte(`{"source":"https://example.com/a","displayAt":123}`)))
}

// inlineManifestJSON is the §3.6 example item manifest, envelope complete.
const inlineManifestJSON = `{"refVersion":"0.1.0","id":"ref-9d26ecb3","created":"2026-07-28T00:00:00Z","locale":"en",
	"metadata":{"title":"Pre-Process","artists":[{"name":"Casey Reas","id":""}],
	"thumbnails":{"default":{"uri":"https://example.com/thumb.png","w":1200,"h":900}}}}`

// §3.6 requires the unmodified ref-manifest schema to be applied to inlineManifest on both
// composed paths. The single-item path is the one that silently ignored it before the spec's
// PlaylistItemExtension refactor, so each case is asserted on both.
func TestInlineManifest_bothComposedPaths(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		manifest string
		wantErr  bool
	}{
		{"valid", inlineManifestJSON, false},
		{"thumbnail_without_dimensions", `{"refVersion":"0.1.0","id":"r","created":"2026-07-28T00:00:00Z","locale":"en",
			"metadata":{"thumbnails":{"default":{"uri":"https://example.com/t.png"}}}}`, false},
		{"missing_locale", `{"refVersion":"0.1.0","id":"r","created":"2026-07-28T00:00:00Z"}`, true},
		{"bad_refVersion", `{"refVersion":"0.1","id":"r","created":"2026-07-28T00:00:00Z","locale":"en"}`, true},
		{"zero_width_thumbnail", `{"refVersion":"0.1.0","id":"r","created":"2026-07-28T00:00:00Z","locale":"en",
			"metadata":{"thumbnails":{"default":{"uri":"https://example.com/t.png","w":0,"h":900}}}}`, true},
		{"not_an_object", `"https://example.com/manifest.json"`, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			item := fmt.Sprintf(`{"source":"https://a","inlineManifest":%s}`, tc.manifest)
			full := fmt.Sprintf(`{"dpVersion":"1.1.0","title":"x","items":[%s],%s}`, item, playlistSigBlock)
			for path, err := range map[string]error{
				"item":     PlaylistItemWithPlaylistsExtension([]byte(item)),
				"playlist": PlaylistWithPlaylistsExtension([]byte(full)),
			} {
				if tc.wantErr {
					assertErrValidation(t, err)
				} else if err != nil {
					t.Fatalf("%s path: %v", path, err)
				}
			}
		})
	}
}

// Core DP-1 tolerates unknown fields: an inlineManifest (even a broken one) must not make a
// document invalid on the core-only path, which knows nothing about the playlists extension.
func TestInlineManifest_ignoredByCoreSchema(t *testing.T) {
	t.Parallel()
	item := `{"source":"https://a","inlineManifest":{"refVersion":"nope"}}`
	if err := PlaylistItem([]byte(item)); err != nil {
		t.Fatal(err)
	}
	doc := fmt.Sprintf(`{"dpVersion":"1.1.0","title":"x","items":[%s],%s}`, item, playlistSigBlock)
	if err := Playlist([]byte(doc)); err != nil {
		t.Fatal(err)
	}
}

// Thumbnail required was relaxed to ["uri"] (core changelog 2026-08-12).
func TestRefManifest_thumbnailDimensionsOptional(t *testing.T) {
	t.Parallel()
	base := `{"refVersion":"0.1.0","id":"r","created":"2025-01-01T00:00:00Z","locale":"en",
		"metadata":{"thumbnails":{"default":{"uri":"https://example.com/t.png"%s}}}}`
	for _, extra := range []string{"", `,"w":320`, `,"h":180`, `,"w":320,"h":180`} {
		if err := RefManifest([]byte(fmt.Sprintf(base, extra))); err != nil {
			t.Fatalf("thumbnail%q: %v", extra, err)
		}
	}
	// Still constrained when present.
	assertErrValidation(t, RefManifest([]byte(fmt.Sprintf(base, `,"w":0`))))
	assertErrValidation(t, RefManifest([]byte(fmt.Sprintf(base, `,"h":-1`))))
	// uri itself remains required.
	assertErrValidation(t, RefManifest([]byte(`{"refVersion":"0.1.0","id":"r","created":"2025-01-01T00:00:00Z","locale":"en",
		"metadata":{"thumbnails":{"default":{"w":320,"h":180}}}}`)))
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
