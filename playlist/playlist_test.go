package playlist

import (
	"encoding/json"
	"testing"
)

func TestPlaylist_JSONRoundTrip(t *testing.T) {
	t.Parallel()
	tru := true
	p := Playlist{
		DPVersion: "1.1.0",
		ID:        "385f79b6-a45f-4c1c-8080-e93a192adccc",
		Title:     "T",
		Defaults: &Defaults{
			Display: &DisplayPrefs{Scaling: "fit", Autoplay: &tru},
			License: "open",
		},
		Items: []PlaylistItem{
			{
				Source:  "https://a",
				License: "token",
				Display: &DisplayPrefs{Scaling: "fill"},
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
}
