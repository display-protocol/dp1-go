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
	// An undecodable interaction block now leaves no trace: presence-based merging allocates
	// the shell only for keys it is about to write, so an unusable block cannot turn an
	// otherwise-empty DisplayPrefs into a non-empty one. Before, it left an empty shell behind.
	if dst.Interaction != nil {
		t.Fatalf("invalid interaction JSON must apply nothing, got %+v", dst.Interaction)
	}
}

func Test_applyInteractionJSON_absentKeysLeaveLowerLayer(t *testing.T) {
	t.Parallel()
	dst := playlist.DisplayPrefs{Interaction: &playlist.InteractionPrefs{
		Keyboard: []string{"KeyA"},
		Mouse:    &playlist.MousePrefs{Click: true},
	}}
	// An interaction object that mentions neither key must not disturb what is already there.
	applyInteractionJSON(&dst, json.RawMessage(`{}`))
	if len(dst.Interaction.Keyboard) != 1 || !dst.Interaction.Mouse.Click {
		t.Fatalf("empty interaction object overwrote lower layer: %+v", dst.Interaction)
	}
	// An empty mouse object is presence without keys: same rule one level down.
	applyInteractionJSON(&dst, json.RawMessage(`{"mouse":{}}`))
	if !dst.Interaction.Mouse.Click {
		t.Fatalf("empty mouse object cleared click: %+v", dst.Interaction.Mouse)
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

// F2 regression: two manifests stack, so the higher-precedence one must override only the keys
// it actually carries. A ref manifest setting mouse.scroll must not erase the inline copy's
// mouse.click, and a present-but-empty keyboard must win over a non-empty lower one.
func TestDisplayForItem_nestedInteractionAcrossInlineAndRef(t *testing.T) {
	t.Parallel()
	withInteraction := func(id, interaction string) *refmanifest.Manifest {
		m := manifestWithScaling(id, "")
		m.Controls.Display.Interaction = json.RawMessage(interaction)
		return m
	}
	inline := withInteraction("inline", `{"keyboard":["KeyA"],"mouse":{"click":true,"hover":true}}`)
	ref := withInteraction("remote", `{"keyboard":[],"mouse":{"scroll":true,"hover":false}}`)
	item := playlist.PlaylistItem{Source: "https://x", InlineManifest: rawManifest(t, inline)}

	out, err := DisplayForItem(nil, ref, item)
	if err != nil {
		t.Fatal(err)
	}
	if out == nil || out.Interaction == nil || out.Interaction.Mouse == nil {
		t.Fatalf("got %+v", out)
	}
	m := out.Interaction.Mouse
	if !m.Click {
		t.Error("mouse.click from the inline manifest was cleared by a ref that never mentions it")
	}
	if !m.Scroll {
		t.Error("mouse.scroll from the ref manifest was not applied")
	}
	if m.Hover {
		t.Error("mouse.hover:false in the ref manifest must override the inline true")
	}
	if m.Drag {
		t.Error("mouse.drag was set by neither manifest")
	}
	if len(out.Interaction.Keyboard) != 0 {
		t.Errorf("an explicit empty keyboard must win, got %v", out.Interaction.Keyboard)
	}
}
