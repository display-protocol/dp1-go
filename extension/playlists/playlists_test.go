package playlists

import (
	"encoding/json"
	"testing"

	"github.com/display-protocol/dp1-go/extension/identity"
)

func TestOverlay_JSONRoundTrip(t *testing.T) {
	t.Parallel()
	o := Overlay{
		Curators: []identity.Entity{{Name: "A", Key: "did:key:z"}},
		Summary:  "S",
		DynamicQuery: &DynamicQuery{
			Profile:  "graphql-v1",
			Endpoint: "https://idx.example/gql",
			ResponseMapping: ResponseMapping{
				ItemsPath:  "data.items",
				ItemSchema: "dp1/1.1",
				ItemMap:    map[string]string{"id": "_id"},
			},
		},
	}
	b, err := json.Marshal(&o)
	if err != nil {
		t.Fatal(err)
	}
	var out Overlay
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	if out.Summary != o.Summary {
		t.Fatal(out.Summary)
	}
}
