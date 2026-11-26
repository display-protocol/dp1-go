package dp1_test

import (
	"encoding/json"
	"fmt"
	"testing"

	dp1 "github.com/display-protocol/dp1-go"
	"github.com/display-protocol/dp1-go/extension/identity"
	"github.com/display-protocol/dp1-go/merge"
	"github.com/display-protocol/dp1-go/refmanifest"
)

// The schema accepts any spelling of an integer, and validation runs before decode, so
// every integer member must decode what the schema let through — on each document that
// carries it: a ref manifest, an inline manifest, and an Entity inside a playlist.
func TestParseAndValidate_integerSpellings(t *testing.T) {
	t.Parallel()
	const manifest = `{
		"refVersion": "1.1.0",
		"id": "m",
		"created": "2026-01-01T00:00:00Z",
		"locale": "en",
		"metadata": {"title": "T", "thumbnails": {"default": {"uri": "https://a/t.png", "w": 100.0, "h": 1e3}}},
		"controls": {"safety": {"maxCpuPct": 5e1, "maxMemMB": 1024.0}}
	}`
	check := func(t *testing.T, m *refmanifest.Manifest) {
		t.Helper()
		th := m.Metadata.Thumbnails["default"]
		s := m.Controls.Safety
		got := fmt.Sprint(*th.W, *th.H, *s.MaxCPUPct, *s.MaxMemMB)
		if got != "100 1000 50 1024" {
			t.Fatalf("got %s", got)
		}
	}
	t.Run("ref manifest", func(t *testing.T) {
		t.Parallel()
		m, err := dp1.ParseAndValidateRefManifest([]byte(manifest))
		if err != nil {
			t.Fatal(err)
		}
		check(t, m)
	})
	t.Run("playlist", func(t *testing.T) {
		t.Parallel()
		doc := []byte(fmt.Sprintf(`{
			"dpVersion": "1.1.0",
			"title": "T",
			"curators": [{"name": "C", "avatar": {"uri": "https://a/c.png", "w": 64.0}}],
			"items": [{"source": "https://a", "inlineManifest": %s}],
			"signatures": [%s]
		}`, manifest, dummySignatureJSON))
		p, err := dp1.ParseAndValidatePlaylist(doc)
		if err != nil {
			t.Fatal(err)
		}
		if w := p.Curators[0].Avatar.W; w == nil || *w != 64 {
			t.Fatalf("avatar width: %v", w)
		}
		m, err := merge.ManifestForItem(nil, p.Items[0])
		if err != nil {
			t.Fatal(err)
		}
		check(t, m)
	})
}

// The custom decoders replace the default one, so they must keep its behavior for
// everything but the spelling: absent keeps the receiver's value, null clears it.
func TestIntegerMembers_defaultDecoderSemantics(t *testing.T) {
	t.Parallel()
	w, h := 1, 2
	th := identity.Thumbnail{URI: "keep", W: &w, H: &h}
	if err := json.Unmarshal([]byte(`{"h": null}`), &th); err != nil {
		t.Fatal(err)
	}
	if th.URI != "keep" || th.W == nil || *th.W != 1 || th.H != nil {
		t.Fatalf("got %+v", th)
	}
	cpu, mem := 3, 4
	s := refmanifest.SafetyControls{MaxCPUPct: &cpu, MaxMemMB: &mem}
	if err := json.Unmarshal([]byte(`{"maxMemMB": null, "orientation": ["any"]}`), &s); err != nil {
		t.Fatal(err)
	}
	if s.MaxCPUPct == nil || *s.MaxCPUPct != 3 || s.MaxMemMB != nil || len(s.Orientation) != 1 {
		t.Fatalf("got %+v", s)
	}
	if err := json.Unmarshal([]byte(`{"uri": "u", "w": 1.5}`), &identity.Thumbnail{}); err == nil {
		t.Fatal("fraction decoded")
	}
}
