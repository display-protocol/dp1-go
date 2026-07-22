package playlist

import (
	"encoding/json"
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

func TestPlaylist_ScheduleAndDisplayAt(t *testing.T) {
	t.Parallel()

	p := Playlist{
		DPVersion: "1.1.0",
		ID:        "385f79b6-a45f-4c1c-8080-e93a192adccc",
		Title:     "Daily",
		Schedule:  &playlists.Schedule{ByDisplayAt: true},
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
				DisplayAt: "2026-07-21T00:00:00",
			},
			{
				ID:        "3",
				Title:     "Day 2",
				Source:    "https://cdn.example.com/day2.html",
				DisplayAt: "2026-07-22T00:00:00",
			},
			{
				ID:        "4",
				Title:     "Day 3",
				Source:    "https://cdn.example.com/day3.html",
				DisplayAt: "2026-07-23T00:00:00Z",
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

	if out.Schedule == nil || !out.Schedule.ByDisplayAt {
		t.Fatalf("expected schedule.byDisplayAt=true, got: %+v", out.Schedule)
	}

	if len(out.Items) != 4 {
		t.Fatalf("expected 4 items, got %d", len(out.Items))
	}

	expected := []string{"", "2026-07-21T00:00:00", "2026-07-22T00:00:00", "2026-07-23T00:00:00Z"}
	for i, want := range expected {
		if out.Items[i].DisplayAt != want {
			t.Errorf("item[%d].DisplayAt = %q, want %q", i, out.Items[i].DisplayAt, want)
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
				DisplayAt: tc.displayAt,
			}
			b, err := json.Marshal(&item)
			if err != nil {
				t.Fatal(err)
			}
			var out PlaylistItem
			if err := json.Unmarshal(b, &out); err != nil {
				t.Fatal(err)
			}
			if out.DisplayAt != tc.displayAt {
				t.Errorf("got %q, want %q", out.DisplayAt, tc.displayAt)
			}
		})
	}
}
