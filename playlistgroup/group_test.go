package playlistgroup

import (
	"encoding/json"
	"testing"

	"github.com/display-protocol/dp1-go/playlist"
)

func TestGroup_JSONRoundTrip(t *testing.T) {
	t.Parallel()
	g := Group{
		ID:        "385f79b6-a45f-4c1c-8080-e93a192adccc",
		Title:     "Ex",
		Playlists: []string{"https://p.json"},
		Created:   "2025-01-01T00:00:00Z",
		Signatures: []playlist.Signature{{
			Alg: "ed25519", Kid: "did:key:z", Ts: "2025-01-01T00:00:00Z",
			PayloadHash: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
			Role:        playlist.RoleFeed, Sig: "def",
		}},
	}
	b, err := json.Marshal(&g)
	if err != nil {
		t.Fatal(err)
	}
	var out Group
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	if out.Title != g.Title {
		t.Fatal(out.Title)
	}
}
