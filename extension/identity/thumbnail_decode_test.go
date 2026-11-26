package identity

import (
	"encoding/json"
	"testing"
)

// Thumbnail's decoder replaces the default one, so it must keep its behavior for
// everything but the spelling: absent keeps the receiver's value, null clears it.
func TestThumbnailUnmarshal(t *testing.T) {
	t.Parallel()
	w, h := 1, 2
	th := Thumbnail{URI: "keep", W: &w, H: &h}
	if err := json.Unmarshal([]byte(`{"h": null}`), &th); err != nil {
		t.Fatal(err)
	}
	if th.URI != "keep" || th.W == nil || *th.W != 1 || th.H != nil {
		t.Fatalf("got %+v", th)
	}
	if err := json.Unmarshal([]byte(`{"w": 1e2, "h": 90.0}`), &th); err != nil {
		t.Fatal(err)
	}
	if *th.W != 100 || th.H == nil || *th.H != 90 {
		t.Fatalf("got %+v", th)
	}
	if err := json.Unmarshal([]byte(`{"uri": "u", "w": 1.5}`), &Thumbnail{}); err == nil {
		t.Fatal("fraction decoded")
	}
}
