package playlist

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/display-protocol/dp1-go/extension/playlists"
)

func TestPlaylist_JSONRoundTrip(t *testing.T) {
	t.Parallel()
	tru := true
	pre := 5.0
	p := Playlist{
		DPVersion: "1.1.0",
		ID:        "385f79b6-a45f-4c1c-8080-e93a192adccc",
		Title:     "T",
		Note:      &playlists.Note{Text: "Show intro", Duration: &pre},
		Defaults: &Defaults{
			Display: &DisplayPrefs{Scaling: "fit", Autoplay: &tru},
			License: "open",
		},
		Items: []PlaylistItem{
			{
				Source:  "https://a",
				License: "token",
				Display: &DisplayPrefs{Scaling: "fill"},
				Note:    &playlists.Note{Text: "Item card"},
				Provenance: &ProvenanceBlock{
					Type: ProvenanceOnChain,
					Contract: &ProvenanceContract{
						Chain: "evm", Standard: "erc721", Address: "0xabc",
					},
				},
			},
		},
		Signatures: []Signature{{
			Alg: "ed25519", Kid: "did:key:z", Ts: "2025-01-01T00:00:00Z",
			PayloadHash: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			Role:        RoleCurator, Sig: "abc",
		}},
	}
	b, err := json.Marshal(&p)
	if err != nil {
		t.Fatal(err)
	}
	var out Playlist
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	if out.Title != p.Title || len(out.Items) != 1 {
		t.Fatalf("%+v", out)
	}
	if out.Note == nil || out.Note.Text != "Show intro" || out.Note.Duration == nil || *out.Note.Duration != 5 {
		t.Fatalf("playlist note: %+v", out.Note)
	}
	if out.Items[0].Note == nil || out.Items[0].Note.Text != "Item card" {
		t.Fatalf("item note: %+v", out.Items[0].Note)
	}
}

func strPtr(s string) *string { return &s }

func displayAtString(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func TestPlaylist_DisplayAt(t *testing.T) {
	t.Parallel()

	p := Playlist{
		DPVersion: "1.1.0",
		ID:        "385f79b6-a45f-4c1c-8080-e93a192adccc",
		Title:     "Daily",
		Items: []PlaylistItem{
			{
				ID:     "1",
				Title:  "Intro",
				Source: "https://cdn.example.com/intro.html",
			},
			{
				ID:        "2",
				Title:     "Day 1",
				Source:    "https://cdn.example.com/day1.html",
				DisplayAt: strPtr("2026-07-21T00:00:00"),
			},
			{
				ID:        "3",
				Title:     "Day 2",
				Source:    "https://cdn.example.com/day2.html",
				DisplayAt: strPtr("2026-07-22T00:00:00"),
			},
			{
				ID:        "4",
				Title:     "Day 3",
				Source:    "https://cdn.example.com/day3.html",
				DisplayAt: strPtr("2026-07-23T00:00:00Z"),
			},
		},
		Signatures: []Signature{{
			Alg: "ed25519", Kid: "did:key:z", Ts: "2025-01-01T00:00:00Z",
			PayloadHash: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			Role:        RoleCurator, Sig: "abc",
		}},
	}

	b, err := json.Marshal(&p)
	if err != nil {
		t.Fatal(err)
	}

	var out Playlist
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}

	if len(out.Items) != 4 {
		t.Fatalf("expected 4 items, got %d", len(out.Items))
	}

	expected := []string{"", "2026-07-21T00:00:00", "2026-07-22T00:00:00", "2026-07-23T00:00:00Z"}
	for i, want := range expected {
		if displayAtString(out.Items[i].DisplayAt) != want {
			t.Errorf("item[%d].DisplayAt = %q, want %q", i, displayAtString(out.Items[i].DisplayAt), want)
		}
		if want == "" && out.Items[i].DisplayAt != nil {
			t.Errorf("item[%d].DisplayAt want nil (absent), got non-nil", i)
		}
	}
}

func TestPlaylistItem_DisplayAt_Formats(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		displayAt string
	}{
		{"local_datetime", "2026-07-21T00:00:00"},
		{"local_with_frac", "2026-07-21T00:00:00.123"},
		{"absolute_Z", "2026-07-21T00:00:00Z"},
		{"absolute_positive_offset", "2026-07-21T09:00:00+07:00"},
		{"absolute_negative_offset", "2026-07-21T00:00:00-05:00"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			item := PlaylistItem{
				Source:    "https://example.com/a",
				DisplayAt: strPtr(tc.displayAt),
			}
			b, err := json.Marshal(&item)
			if err != nil {
				t.Fatal(err)
			}
			var out PlaylistItem
			if err := json.Unmarshal(b, &out); err != nil {
				t.Fatal(err)
			}
			if displayAtString(out.DisplayAt) != tc.displayAt {
				t.Errorf("got %q, want %q", displayAtString(out.DisplayAt), tc.displayAt)
			}
		})
	}
}

func TestPlaylistItem_DisplayAt_NullAndEmpty(t *testing.T) {
	t.Parallel()

	var nullItem PlaylistItem
	if err := json.Unmarshal([]byte(`{"source":"https://a","displayAt":null}`), &nullItem); err != nil {
		t.Fatal(err)
	}
	if nullItem.DisplayAt != nil {
		t.Fatalf("null displayAt: got %q, want nil", *nullItem.DisplayAt)
	}

	var emptyItem PlaylistItem
	if err := json.Unmarshal([]byte(`{"source":"https://a","displayAt":""}`), &emptyItem); err != nil {
		t.Fatal(err)
	}
	if emptyItem.DisplayAt == nil || *emptyItem.DisplayAt != "" {
		t.Fatalf("empty displayAt: got %#v, want pointer to empty string", emptyItem.DisplayAt)
	}
}

// InlineManifest is the playlists-extension carriage of a full ref manifest (§3.6). Because it
// is raw JSON the wire form survives a round trip byte for byte — including the artist "id": ""
// of the §3.6 example, which a decoded manifest would drop through omitempty and thereby change
// the JCS payload of a signed playlist.
func TestPlaylistItem_InlineManifest_RoundTripIsByteFaithful(t *testing.T) {
	t.Parallel()
	manifest := `{"refVersion":"0.1.0","id":"ref-9d26ecb3","created":"2026-07-28T00:00:00Z","locale":"en",` +
		`"metadata":{"title":"Pre-Process","artists":[{"name":"Casey Reas","id":""}],` +
		`"thumbnails":{"default":{"uri":"https://example.com/thumb.png","w":1200,"h":900},` +
		`"small":{"uri":"https://example.com/thumb-s.png"}}}}`
	wire := []byte(`{"source":"https://example.com/work.html","inlineManifest":` + manifest + `}`)

	var item PlaylistItem
	if err := json.Unmarshal(wire, &item); err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(&item)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(b, wire) {
		t.Fatalf("round trip changed the bytes:\n want %s\n got  %s", wire, b)
	}

	m, err := item.ParseInlineManifest()
	if err != nil {
		t.Fatal(err)
	}
	if m == nil || m.ID != "ref-9d26ecb3" {
		t.Fatalf("inlineManifest: %+v", m)
	}
	th := m.Metadata.Thumbnails
	if th["default"].W == nil || *th["default"].W != 1200 {
		t.Fatalf("default thumbnail width: %+v", th["default"])
	}
	if th["small"].W != nil || th["small"].H != nil {
		t.Fatalf("expected absent dimensions to stay absent: %+v", th["small"])
	}

	bare, err := json.Marshal(&PlaylistItem{Source: "https://example.com/work.html"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(bare), "inlineManifest") {
		t.Fatalf("unset inlineManifest must be omitted: %s", bare)
	}
}

// A core-parsed playlist can carry any JSON here, so decoding must fail loudly rather than
// silently yielding a nil manifest that reads as "no manifest present".
func TestPlaylistItem_ParseInlineManifest_absentAndInvalid(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{"", "null"} {
		item := PlaylistItem{Source: "https://a", InlineManifest: json.RawMessage(raw)}
		m, err := item.ParseInlineManifest()
		if err != nil || m != nil {
			t.Fatalf("%q must read as absent, got %+v err=%v", raw, m, err)
		}
	}
	for _, raw := range []string{`"https://m.example/x.json"`, `[]`, `{"metadata":{"thumbnails":{"default":{"w":"1200"}}}}`} {
		item := PlaylistItem{Source: "https://a", InlineManifest: json.RawMessage(raw)}
		if _, err := item.ParseInlineManifest(); err == nil {
			t.Fatalf("%s must fail to decode", raw)
		}
	}
}
