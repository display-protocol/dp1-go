package merge

import (
	"encoding/json"
	"testing"

	"github.com/display-protocol/dp1-go/playlist"
	"github.com/display-protocol/dp1-go/refmanifest"
)

// --- defaults / ref / empty ---

func TestDisplayForItem_empty(t *testing.T) {
	t.Parallel()
	item := playlist.PlaylistItem{Source: "https://x"}
	out, err := DisplayForItem(nil, nil, item)
	if err != nil {
		t.Fatal(err)
	}
	if out != nil {
		t.Fatalf("expected nil, got %+v", out)
	}
}

func TestDisplayForItem_defaultsOnly(t *testing.T) {
	t.Parallel()
	def := &playlist.Defaults{
		Display: &playlist.DisplayPrefs{Scaling: "fit"},
	}
	item := playlist.PlaylistItem{Source: "https://x"}
	out, err := DisplayForItem(def, nil, item)
	if err != nil {
		t.Fatal(err)
	}
	if out == nil || out.Scaling != "fit" {
		t.Fatalf("got %+v", out)
	}
}

func TestDisplayForItem_defaultsThenItem(t *testing.T) {
	t.Parallel()
	tru := true
	def := &playlist.Defaults{
		Display: &playlist.DisplayPrefs{
			Scaling:  "fit",
			Autoplay: &tru,
		},
	}
	item := playlist.PlaylistItem{
		Source: "https://x",
		Display: &playlist.DisplayPrefs{
			Scaling: "fill",
		},
	}
	out, err := DisplayForItem(def, nil, item)
	if err != nil {
		t.Fatal(err)
	}
	if out == nil || out.Scaling != "fill" {
		t.Fatalf("got %+v", out)
	}
	if out.Autoplay == nil || !*out.Autoplay {
		t.Fatal("expected autoplay inherited from defaults")
	}
}

func TestDisplayForItem_refOverlay(t *testing.T) {
	t.Parallel()
	ref := &refmanifest.Manifest{
		RefVersion: "0.1.0",
		ID:         "r1",
		Created:    "2025-01-01T00:00:00Z",
		Locale:     "en",
		Controls: &refmanifest.Controls{
			Display: &refmanifest.DisplayControls{
				Scaling: "stretch",
			},
		},
	}
	item := playlist.PlaylistItem{Source: "https://x"}
	out, err := DisplayForItem(nil, ref, item)
	if err != nil {
		t.Fatal(err)
	}
	if out == nil || out.Scaling != "stretch" {
		t.Fatalf("got %+v", out)
	}
}

// --- override vs item precedence ---

func TestDisplayForItem_override(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		override map[string]any
		display  *playlist.DisplayPrefs
		want     string
	}{
		{
			name: "json_sets_scaling",
			override: map[string]any{
				"display": map[string]any{"scaling": "auto"},
			},
			display: nil,
			want:    "auto",
		},
		{
			name: "empty_display_object_item_wins",
			override: map[string]any{
				"display": map[string]any{},
			},
			display: &playlist.DisplayPrefs{Scaling: "fill"},
			want:    "fill",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ov, err := json.Marshal(tc.override)
			if err != nil {
				t.Fatal(err)
			}
			item := playlist.PlaylistItem{
				Source:   "https://x",
				Override: ov,
				Display:  tc.display,
			}
			out, err := DisplayForItem(nil, nil, item)
			if err != nil {
				t.Fatal(err)
			}
			if out == nil || out.Scaling != tc.want {
				t.Fatalf("got %+v", out)
			}
		})
	}
}

func TestDisplayForItem_overrideThenItem_itemWins(t *testing.T) {
	t.Parallel()
	ov, _ := json.Marshal(map[string]any{
		"display": map[string]any{"scaling": "stretch"},
	})
	item := playlist.PlaylistItem{
		Source:   "https://x",
		Override: ov,
		Display:  &playlist.DisplayPrefs{Scaling: "fill"},
	}
	out, err := DisplayForItem(nil, nil, item)
	if err != nil {
		t.Fatal(err)
	}
	if out == nil || out.Scaling != "fill" {
		t.Fatalf("expected item-local display to win over override, got %+v", out)
	}
}

func TestDisplayForItem_refThenOverrideThenItem(t *testing.T) {
	t.Parallel()
	ref := &refmanifest.Manifest{
		RefVersion: "0.1.0",
		ID:         "r",
		Created:    "2025-01-01T00:00:00Z",
		Locale:     "en",
		Controls: &refmanifest.Controls{
			Display: &refmanifest.DisplayControls{Scaling: "fit"},
		},
	}
	ov, _ := json.Marshal(map[string]any{
		"display": map[string]any{"scaling": "fill"},
	})
	item := playlist.PlaylistItem{
		Source:   "https://x",
		Override: ov,
		Display:  &playlist.DisplayPrefs{Scaling: "auto"},
	}
	out, err := DisplayForItem(nil, ref, item)
	if err != nil {
		t.Fatal(err)
	}
	if out == nil || out.Scaling != "auto" {
		t.Fatalf("got %+v", out)
	}
}

// --- full stack + applyDisplayJSON edge ---

func TestDisplayForItem_fullOverlay(t *testing.T) {
	t.Parallel()
	fal := false
	tru := true
	def := &playlist.Defaults{
		Display: &playlist.DisplayPrefs{
			Scaling:  "fit",
			Autoplay: &fal,
			Interaction: &playlist.InteractionPrefs{
				Keyboard: []string{"KeyA"},
				Mouse:    &playlist.MousePrefs{Click: true},
			},
			UserOverrides: map[string]bool{"scaling": true},
		},
	}
	ref := &refmanifest.Manifest{
		RefVersion: "0.1.0",
		ID:         "r",
		Created:    "2025-01-01T00:00:00Z",
		Locale:     "en",
		Controls: &refmanifest.Controls{
			Display: &refmanifest.DisplayControls{
				Scaling:    "fill",
				Margin:     json.RawMessage(`"5%"`),
				Background: "#111111",
				Autoplay:   &tru,
				Loop:       &tru,
				Interaction: mustJSON(t, map[string]any{
					"keyboard": []string{"Space"},
					"mouse":    map[string]any{"scroll": true, "drag": true, "hover": true},
				}),
			},
		},
	}
	item := playlist.PlaylistItem{
		Source: "https://x",
		Display: &playlist.DisplayPrefs{
			Scaling: "auto",
			Interaction: &playlist.InteractionPrefs{
				Keyboard: []string{"Enter"},
				Mouse:    &playlist.MousePrefs{Hover: true},
			},
			UserOverrides: map[string]bool{"margin": true},
		},
	}
	out, err := DisplayForItem(def, ref, item)
	if err != nil {
		t.Fatal(err)
	}
	if out == nil {
		t.Fatal("nil")
	}
	if out.Scaling != "auto" {
		t.Fatal(out.Scaling)
	}
	if out.Background != "#111111" {
		t.Fatal(out.Background)
	}
	if out.Loop == nil || !*out.Loop {
		t.Fatal("loop")
	}
	if len(out.UserOverrides) < 2 {
		t.Fatal(out.UserOverrides)
	}
}

func Test_applyDisplayJSON_invalidInteractionIgnored(t *testing.T) {
	t.Parallel()
	var dst playlist.DisplayPrefs
	src := refmanifest.DisplayControls{
		Scaling:     "stretch",
		Interaction: json.RawMessage(`{"keyboard":42}`),
	}
	applyDisplayJSON(&dst, &src)
	if dst.Scaling != "stretch" {
		t.Fatal(dst.Scaling)
	}
	if dst.Interaction == nil {
		t.Fatal("expected interaction shell when controls include interaction JSON")
	}
	if len(dst.Interaction.Keyboard) != 0 || dst.Interaction.Mouse != nil {
		t.Fatal("expected no interaction fields applied when JSON is invalid")
	}
}

func mustJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// --- errors ---

func TestDisplayForItem_badOverride(t *testing.T) {
	t.Parallel()
	item := playlist.PlaylistItem{
		Source:   "https://x",
		Override: json.RawMessage(`not-json`),
	}
	_, err := DisplayForItem(nil, nil, item)
	if err == nil {
		t.Fatal("expected error")
	}
}

// --- inline manifest (playlists extension §3.6) ---

// rawManifest renders a manifest to the wire form PlaylistItem.InlineManifest holds.
func rawManifest(t *testing.T, m *refmanifest.Manifest) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func manifestWithScaling(id, scaling string) *refmanifest.Manifest {
	return &refmanifest.Manifest{
		RefVersion: "0.1.0",
		ID:         id,
		Created:    "2025-01-01T00:00:00Z",
		Locale:     "en",
		Controls: &refmanifest.Controls{
			Display: &refmanifest.DisplayControls{Scaling: scaling},
		},
	}
}

func TestDisplayForItem_inlineManifestOverlay(t *testing.T) {
	t.Parallel()
	item := playlist.PlaylistItem{
		Source:         "https://x",
		InlineManifest: rawManifest(t, manifestWithScaling("inline", "stretch")),
	}
	out, err := DisplayForItem(nil, nil, item)
	if err != nil {
		t.Fatal(err)
	}
	if out == nil || out.Scaling != "stretch" {
		t.Fatalf("got %+v", out)
	}
}

// Resolution order is defaults → inlineManifest → ref → override → item-local, so a fetched
// ref manifest overrides the inline copy on the same key path.
func TestDisplayForItem_refWinsOverInlineManifest(t *testing.T) {
	t.Parallel()
	loop := true
	inline := manifestWithScaling("inline", "stretch")
	inline.Controls.Display.Loop = &loop
	item := playlist.PlaylistItem{Source: "https://x", InlineManifest: rawManifest(t, inline)}

	out, err := DisplayForItem(nil, manifestWithScaling("remote", "fill"), item)
	if err != nil {
		t.Fatal(err)
	}
	if out == nil || out.Scaling != "fill" {
		t.Fatalf("ref must win on scaling, got %+v", out)
	}
	// Keys the ref manifest leaves unset still come from the inline copy.
	if out.Loop == nil || !*out.Loop {
		t.Fatalf("expected loop from inline manifest, got %+v", out)
	}
}

func TestDisplayForItem_itemLocalWinsOverInlineManifest(t *testing.T) {
	t.Parallel()
	item := playlist.PlaylistItem{
		Source:         "https://x",
		InlineManifest: rawManifest(t, manifestWithScaling("inline", "stretch")),
		Display:        &playlist.DisplayPrefs{Scaling: "fit"},
	}
	out, err := DisplayForItem(nil, nil, item)
	if err != nil {
		t.Fatal(err)
	}
	if out == nil || out.Scaling != "fit" {
		t.Fatalf("got %+v", out)
	}
}

func TestManifestForItem(t *testing.T) {
	t.Parallel()
	remote := manifestWithScaling("remote", "fill")
	item := playlist.PlaylistItem{Source: "https://x", InlineManifest: rawManifest(t, manifestWithScaling("inline", "stretch"))}

	got, err := ManifestForItem(remote, item)
	if err != nil || got != remote {
		t.Fatalf("ref must win, got %+v err=%v", got, err)
	}
	got, err = ManifestForItem(nil, item)
	if err != nil || got == nil || got.ID != "inline" {
		t.Fatalf("expected inline fallback, got %+v err=%v", got, err)
	}
	got, err = ManifestForItem(nil, playlist.PlaylistItem{Source: "https://x"})
	if err != nil || got != nil {
		t.Fatalf("expected nil, got %+v err=%v", got, err)
	}
	// A core-parsed playlist can carry anything under inlineManifest; the decode error is the
	// only place that surfaces, and it must not be mistaken for "no manifest".
	got, err = ManifestForItem(nil, playlist.PlaylistItem{Source: "https://x", InlineManifest: json.RawMessage(`"not-a-manifest"`)})
	if err == nil || got != nil {
		t.Fatalf("expected decode error, got %+v err=%v", got, err)
	}
	got, err = ManifestForItem(nil, playlist.PlaylistItem{Source: "https://x", InlineManifest: json.RawMessage(`null`)})
	if err != nil || got != nil {
		t.Fatalf("JSON null must read as absent, got %+v err=%v", got, err)
	}
}

// A malformed inline manifest is fatal only when nothing can replace it. With no ref fetched
// there is no fallback, so the error must surface — swallowing it would hand a caller on the
// core-only path prefs quietly missing the inline layer. With a ref in hand §3.6 makes the
// fetched manifest authoritative and the inline copy goes unread, so it must not block
// rendering a conforming playlist.
func TestDisplayForItem_badInlineManifest(t *testing.T) {
	t.Parallel()
	item := playlist.PlaylistItem{
		Source:         "https://x",
		InlineManifest: json.RawMessage(`"https://m.example/x.json"`),
	}

	out, err := DisplayForItem(nil, nil, item)
	if err == nil || out != nil {
		t.Fatalf("no ref: expected decode error, got %+v err=%v", out, err)
	}

	out, err = DisplayForItem(nil, manifestWithScaling("remote", "fill"), item)
	if err != nil {
		t.Fatalf("an unread inline fallback must not block the authoritative ref: %v", err)
	}
	if out == nil || out.Scaling != "fill" {
		t.Fatalf("expected the ref manifest to be applied, got %+v", out)
	}
}
