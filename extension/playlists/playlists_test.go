package playlists

import (
	"encoding/json"
	"testing"

	"github.com/display-protocol/dp1-go/extension/identity"
)

func TestOverlay_JSONRoundTrip(t *testing.T) {
	t.Parallel()
	dur := 20.0
	o := Overlay{
		Note:       &Note{Text: "Interlude", Duration: &dur},
		Curators:   []identity.Entity{{Name: "A", Key: "did:key:z"}},
		Summary:    "S",
		Satellites: []Satellite{{Playlist: "https://feed.example/p/burn", Kind: "burn"}},
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
	if out.Note == nil || out.Note.Text != "Interlude" || out.Note.Duration == nil || *out.Note.Duration != 20 {
		t.Fatalf("note: %+v", out.Note)
	}
	if len(out.Satellites) != 1 || out.Satellites[0].Playlist != "https://feed.example/p/burn" || out.Satellites[0].Kind != "burn" {
		t.Fatalf("satellites: %+v", out.Satellites)
	}
}

// TestSatellite_UnmarshalJSON_exactKey locks in that each Satellite member is read by its exact
// JSON key: the playlists-extension schema permits properties it does not describe, so a
// document carrying both "playlist" and an unrecognized "Playlist" (or any other case variant
// of playlist/kind/title/note) validates — the second member is merely unknown. Without
// exact-key lookup, encoding/json's case-insensitive fallback would let it silently overwrite
// the validated value, the same risk PlaylistItem's content-rating fields avoid.
func TestSatellite_UnmarshalJSON_exactKey(t *testing.T) {
	t.Parallel()
	doc := []byte(`{"playlist":"https://feed.example/burn","kind":"burn","title":"Burn",
		"note":"n","Playlist":"https://other.example/catalog","Kind":"catalog","Title":"T","Note":"N"}`)
	var s Satellite
	if err := json.Unmarshal(doc, &s); err != nil {
		t.Fatal(err)
	}
	if s.Playlist != "https://feed.example/burn" || s.Kind != "burn" || s.Title != "Burn" || s.Note != "n" {
		t.Fatalf("exact-key members overwritten by case variants: %+v", s)
	}
}

// TestSatellite_UnmarshalJSON_lonelyWrongCaseKeyIgnored covers the sibling case the dual-key
// test above cannot: a document with ONLY a wrong-case key and no exact-case counterpart at
// all. A seed pass that trusts a plain json.Unmarshal into the typed struct would let
// encoding/json's own case-insensitive fallback match it there, with nothing in the exact-key
// loop to overwrite it since the exact key never appears.
func TestSatellite_UnmarshalJSON_lonelyWrongCaseKeyIgnored(t *testing.T) {
	t.Parallel()
	doc := []byte(`{"playlist":"https://feed.example/burn","kind":"burn","Title":"spoofed","Note":"spoofed"}`)
	var s Satellite
	if err := json.Unmarshal(doc, &s); err != nil {
		t.Fatal(err)
	}
	if s.Title != "" || s.Note != "" {
		t.Fatalf("a lone wrong-case key must not populate the field: %+v", s)
	}
}

// TestSatellite_UnmarshalJSON_wronglyTypedCaseVariantDoesNotFailDecode covers a further sibling:
// a lone wrong-case key whose value is not even a string. A shape check that runs any plain
// json.Unmarshal into a typed struct — even just to validate shape, discarding the result —
// would have that non-string value case-fold onto the typed field and fail the whole decode,
// rejecting a document the schema accepts (Satellite has no additionalProperties:false).
func TestSatellite_UnmarshalJSON_wronglyTypedCaseVariantDoesNotFailDecode(t *testing.T) {
	t.Parallel()
	doc := []byte(`{"playlist":"https://feed.example/burn","kind":"burn","Title":5,"Note":[1,2]}`)
	var s Satellite
	if err := json.Unmarshal(doc, &s); err != nil {
		t.Fatalf("a wrong-case, wrongly-typed unknown property must not fail the decode: %v", err)
	}
	if s.Playlist != "https://feed.example/burn" || s.Kind != "burn" || s.Title != "" || s.Note != "" {
		t.Fatalf("unexpected result: %+v", s)
	}
}
