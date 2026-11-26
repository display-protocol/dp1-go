package dp1_test

import (
	"fmt"
	"testing"

	dp1 "github.com/display-protocol/dp1-go"
)

// The schema choice is observable through what each document is held to: the same cases the
// core parser accepts or rejects differently.
func TestParseAndValidatePlaylistByFields(t *testing.T) {
	t.Parallel()
	const dq = `"dynamicQuery": {"profile": "https-json-v1", "endpoint": "https://i.example/q",
		"responseMapping": {"itemsPath": "items", "itemSchema": "dp1/1.1"}}`
	item := func(extra string) string { return `"items": [{"source": "https://a"` + extra + `}]` }
	cases := []struct {
		name    string
		members string
		ok      bool
	}{
		{"core only", item(``), true},
		{"empty items with dynamicQuery", `"items": [], ` + dq, true},
		{"empty items alone", `"items": []`, false},
		{"null items", `"items": null, ` + dq, false},
		{"playlists field malformed", item(``) + `, "openAt": "nope"`, false},
		{"playlists field null", item(``) + `, "note": null`, false},
		{"case variant selects nothing", item(``) + `, "OpenAt": "nope"`, true},
		{"item playlists field malformed", item(`, "inlineManifest": 5`), false},
		{"item rating malformed", item(`, "contentRating": 5`), false},
		{"item rating valid", item(`, "contentRating": "mature"`), true},
		{"both: rating malformed", item(`, "contentRating": 5`) + `, "summary": "s"`, false},
		{"both: playlists malformed", item(`, "contentRating": "mature"`) + `, "openAt": "nope"`, false},
		{"both valid", item(`, "contentRating": "mature"`) + `, "summary": "s"`, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			doc := []byte(fmt.Sprintf(`{"dpVersion": "1.1.0", "title": "T", %s, "signatures": [%s]}`,
				tc.members, dummySignatureJSON))
			p, err := dp1.ParseAndValidatePlaylistByFields(doc)
			if (err == nil) != tc.ok || (p != nil) != tc.ok {
				t.Fatalf("ok=%v, got err %v", tc.ok, err)
			}
		})
	}
	for _, doc := range []string{`not json`, `[]`, `"x"`} {
		if _, err := dp1.ParseAndValidatePlaylistByFields([]byte(doc)); err == nil {
			t.Errorf("%s: accepted", doc)
		}
	}
}
