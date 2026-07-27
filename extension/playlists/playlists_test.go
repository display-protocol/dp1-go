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
		Note:     &Note{Text: "Interlude", Duration: &dur},
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
	if out.Note == nil || out.Note.Text != "Interlude" || out.Note.Duration == nil || *out.Note.Duration != 20 {
		t.Fatalf("note: %+v", out.Note)
	}
}

func TestSchedule_JSONRoundTrip(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		schedule    Schedule
		wantJSON    string
		wantEnabled bool
	}{
		{
			name:        "byDisplayAt_true",
			schedule:    Schedule{ByDisplayAt: true},
			wantJSON:    `{"byDisplayAt":true}`,
			wantEnabled: true,
		},
		{
			name:        "byDisplayAt_false",
			schedule:    Schedule{ByDisplayAt: false},
			wantJSON:    `{}`,
			wantEnabled: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			b, err := json.Marshal(&tc.schedule)
			if err != nil {
				t.Fatal(err)
			}
			if string(b) != tc.wantJSON {
				t.Errorf("got JSON %s, want %s", string(b), tc.wantJSON)
			}
			var out Schedule
			if err := json.Unmarshal(b, &out); err != nil {
				t.Fatal(err)
			}
			if out.ByDisplayAt != tc.wantEnabled {
				t.Errorf("got ByDisplayAt %v, want %v", out.ByDisplayAt, tc.wantEnabled)
			}
		})
	}
}
