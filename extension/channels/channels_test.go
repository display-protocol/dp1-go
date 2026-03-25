package channels

import (
	"encoding/json"
	"testing"

	"github.com/display-protocol/dp1-go/extension/identity"
	"github.com/display-protocol/dp1-go/playlist"
)

func TestChannel_JSONRoundTrip(t *testing.T) {
	t.Parallel()
	c := Channel{
		ID:        "385f79b6-a45f-4c1c-8080-e93a192adccc",
		Slug:      "s",
		Title:     "C",
		Version:   "1.0.0",
		Created:   "2025-01-01T00:00:00Z",
		Playlists: []string{"https://a"},
		Curators:  []identity.Entity{{Name: "A", Key: "did:key:z"}},
		Publisher: &identity.Entity{Name: "P", Key: "did:key:y"},
		Signatures: []playlist.Signature{{
			Alg: "ed25519", Kid: "did:key:z", Ts: "2025-01-01T00:00:00Z",
			PayloadHash: "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
			Role:        playlist.RoleCurator, Sig: "ghi",
		}},
	}
	b, err := json.Marshal(&c)
	if err != nil {
		t.Fatal(err)
	}
	var out Channel
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	if out.Title != c.Title {
		t.Fatal(out.Title)
	}
}
