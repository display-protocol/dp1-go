package identity

import (
	"encoding/json"
	"testing"
)

func TestEntity_JSONRoundTrip(t *testing.T) {
	t.Parallel()
	e := Entity{Name: "N", Key: "did:key:z", URL: "https://x"}
	b, err := json.Marshal(&e)
	if err != nil {
		t.Fatal(err)
	}
	var out Entity
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	if out.Name != e.Name || out.Key != e.Key {
		t.Fatalf("%+v", out)
	}
}
