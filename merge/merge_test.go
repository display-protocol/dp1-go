package merge

import (
	"encoding/json"
	"strings"
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
				Keyboard: playlist.Keys("KeyA"),
				Mouse:    &playlist.MousePrefs{Click: &tru},
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
				Keyboard: playlist.Keys("Enter"),
				Mouse:    &playlist.MousePrefs{Hover: &tru},
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
	tru := true
	dst := playlist.DisplayPrefs{Interaction: &playlist.InteractionPrefs{
		Keyboard: playlist.Keys("KeyA"),
		Mouse:    &playlist.MousePrefs{Click: &tru},
	}}
	// An interaction object that mentions neither key must not disturb what is already there.
	applyInteractionJSON(&dst, json.RawMessage(`{}`))
	if keyCount(dst.Interaction.Keyboard) != 1 || !boolVal(dst.Interaction.Mouse.Click) {
		t.Fatalf("empty interaction object overwrote lower layer: %+v", dst.Interaction)
	}
	// An empty mouse object is presence without keys: same rule one level down.
	applyInteractionJSON(&dst, json.RawMessage(`{"mouse":{}}`))
	if !boolVal(dst.Interaction.Mouse.Click) {
		t.Fatalf("empty mouse object cleared click: %+v", dst.Interaction.Mouse)
	}
}

// keyCount reads the merged keyboard list; nil (no layer set it) reads as no keys.
func keyCount(p *[]string) int {
	if p == nil {
		return 0
	}
	return len(*p)
}

// boolVal reads an optional interaction toggle: nil (no layer set it) reads as off.
func boolVal(p *bool) bool { return p != nil && *p }

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
	if !boolVal(m.Click) {
		t.Error("mouse.click from the inline manifest was cleared by a ref that never mentions it")
	}
	if !boolVal(m.Scroll) {
		t.Error("mouse.scroll from the ref manifest was not applied")
	}
	if boolVal(m.Hover) {
		t.Error("mouse.hover:false in the ref manifest must override the inline true")
	}
	if boolVal(m.Drag) {
		t.Error("mouse.drag was set by neither manifest")
	}
	if keyCount(out.Interaction.Keyboard) != 0 {
		t.Errorf("an explicit empty keyboard must win, got %v", out.Interaction.Keyboard)
	}
}

// Review F1 regression: the layers above a manifest must be able to switch an interaction off,
// not only on. Item-local display and override.display both carry explicit false and an empty
// keyboard through to the result, over anything an inline or fetched manifest enabled.
func TestDisplayForItem_itemLocalRevokesManifestInteraction(t *testing.T) {
	t.Parallel()
	fal := false
	manifest := func(id string) *refmanifest.Manifest {
		m := manifestWithScaling(id, "")
		m.Controls.Display.Interaction = json.RawMessage(
			`{"keyboard":["KeyA","Space"],"mouse":{"click":true,"scroll":true}}`)
		return m
	}

	t.Run("item_display", func(t *testing.T) {
		t.Parallel()
		item := playlist.PlaylistItem{
			Source:         "https://x",
			InlineManifest: rawManifest(t, manifest("inline")),
			Display: &playlist.DisplayPrefs{
				Interaction: &playlist.InteractionPrefs{
					Keyboard: playlist.Keys(),
					Mouse:    &playlist.MousePrefs{Click: &fal},
				},
			},
		}
		out, err := DisplayForItem(nil, manifest("remote"), item)
		if err != nil {
			t.Fatal(err)
		}
		if boolVal(out.Interaction.Mouse.Click) {
			t.Error("item-local click:false must switch off what the manifests enabled")
		}
		if !boolVal(out.Interaction.Mouse.Scroll) {
			t.Error("scroll was not mentioned by the item and must survive from the manifests")
		}
		if keyCount(out.Interaction.Keyboard) != 0 {
			t.Errorf("item-local empty keyboard must revoke the manifest keys, got %v", out.Interaction.Keyboard)
		}
	})

	t.Run("override_display", func(t *testing.T) {
		t.Parallel()
		item := playlist.PlaylistItem{
			Source:         "https://x",
			InlineManifest: rawManifest(t, manifest("inline")),
			Override: json.RawMessage(
				`{"display":{"interaction":{"keyboard":[],"mouse":{"click":false,"hover":true}}}}`),
		}
		out, err := DisplayForItem(nil, nil, item)
		if err != nil {
			t.Fatal(err)
		}
		if boolVal(out.Interaction.Mouse.Click) {
			t.Error("override click:false must switch off what the inline manifest enabled")
		}
		if !boolVal(out.Interaction.Mouse.Hover) {
			t.Error("override hover:true was dropped")
		}
		if !boolVal(out.Interaction.Mouse.Scroll) {
			t.Error("scroll was not mentioned by the override and must survive")
		}
		if keyCount(out.Interaction.Keyboard) != 0 {
			t.Errorf("override empty keyboard must revoke the manifest keys, got %v", out.Interaction.Keyboard)
		}
	})
}

// Defaults are the lowest layer, so overlays must not write back through the pointers they
// inherit from it — a second item would otherwise start from a mutated baseline.
func TestDisplayForItem_resultNeverSharesPointersWithDefaults(t *testing.T) {
	t.Parallel()
	tru := true
	def := &playlist.Defaults{Display: &playlist.DisplayPrefs{
		Autoplay: &tru,
		Loop:     &tru,
		Margin:   json.RawMessage(`"5%"`),
		Interaction: &playlist.InteractionPrefs{
			Keyboard: playlist.Keys("KeyA"),
			Mouse:    &playlist.MousePrefs{Click: &tru},
		},
	}}
	item := playlist.PlaylistItem{Source: "https://x"}

	// Assert pointer identity, not values: the hazard is a player writing through the returned
	// prefs and mutating the defaults every later item starts from. Comparing values passes
	// whether or not the copy happened, which is what made the earlier version of this test
	// survive reverting the copy.
	out, err := DisplayForItem(def, nil, item)
	if err != nil {
		t.Fatal(err)
	}
	d := def.Display
	if out.Autoplay == d.Autoplay {
		t.Error("Autoplay pointer shared with the playlist defaults")
	}
	if out.Loop == d.Loop {
		t.Error("Loop pointer shared with the playlist defaults")
	}
	if len(out.Margin) > 0 && &out.Margin[0] == &d.Margin[0] {
		t.Error("Margin backing array shared with the playlist defaults")
	}
	if out.Interaction == d.Interaction {
		t.Error("Interaction pointer shared with the playlist defaults")
	}
	if out.Interaction.Keyboard == d.Interaction.Keyboard {
		t.Error("Keyboard pointer shared with the playlist defaults")
	}
	if out.Interaction.Mouse == d.Interaction.Mouse {
		t.Error("Mouse pointer shared with the playlist defaults")
	}
	if out.Interaction.Mouse.Click == d.Interaction.Mouse.Click {
		t.Error("Mouse.Click pointer shared with the playlist defaults")
	}

	// And the write itself, end to end: two items resolved from one Defaults must not see each
	// other's edits.
	*out.Autoplay = false
	*out.Interaction.Mouse.Click = false
	second, err := DisplayForItem(def, nil, item)
	if err != nil {
		t.Fatal(err)
	}
	if !boolVal(second.Autoplay) || !boolVal(second.Interaction.Mouse.Click) {
		t.Fatal("writing through one item's prefs changed the defaults for the next")
	}
}

// The merged prefs get serialized by players (caching, flattening defaults into items, shipping
// to a device), so assert on the wire form, not on a helper that collapses nil and empty. An
// empty keyboard must survive as [] — "keyboard": null is rejected by the core playlist schema
// and decodes back to absence, silently undoing the revocation.
func TestDisplayForItem_emptyKeyboardEncodesAsArray(t *testing.T) {
	t.Parallel()
	sources := map[string]func() (*playlist.Defaults, *refmanifest.Manifest, playlist.PlaylistItem){
		"defaults": func() (*playlist.Defaults, *refmanifest.Manifest, playlist.PlaylistItem) {
			return &playlist.Defaults{Display: &playlist.DisplayPrefs{
				Interaction: &playlist.InteractionPrefs{Keyboard: playlist.Keys()},
			}}, nil, playlist.PlaylistItem{Source: "https://x"}
		},
		"item_display": func() (*playlist.Defaults, *refmanifest.Manifest, playlist.PlaylistItem) {
			return nil, nil, playlist.PlaylistItem{
				Source:  "https://x",
				Display: &playlist.DisplayPrefs{Interaction: &playlist.InteractionPrefs{Keyboard: playlist.Keys()}},
			}
		},
		"override": func() (*playlist.Defaults, *refmanifest.Manifest, playlist.PlaylistItem) {
			return nil, nil, playlist.PlaylistItem{
				Source:   "https://x",
				Override: json.RawMessage(`{"display":{"interaction":{"keyboard":[]}}}`),
			}
		},
		"manifest": func() (*playlist.Defaults, *refmanifest.Manifest, playlist.PlaylistItem) {
			m := manifestWithScaling("r", "")
			m.Controls.Display.Interaction = json.RawMessage(`{"keyboard":[]}`)
			return nil, m, playlist.PlaylistItem{Source: "https://x"}
		},
	}
	for name, build := range sources {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			def, ref, item := build()
			out, err := DisplayForItem(def, ref, item)
			if err != nil {
				t.Fatal(err)
			}
			b, err := json.Marshal(out)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(b), `"keyboard":[]`) {
				t.Fatalf("empty keyboard must encode as an array, got %s", b)
			}
			// And it must still read back as an explicit revocation, not as absence.
			var back playlist.DisplayPrefs
			if err := json.Unmarshal(b, &back); err != nil {
				t.Fatal(err)
			}
			if back.Interaction == nil || back.Interaction.Keyboard == nil {
				t.Fatalf("re-decoded prefs lost the revocation: %s", b)
			}
			if len(*back.Interaction.Keyboard) != 0 {
				t.Fatalf("expected an empty list, got %v", *back.Interaction.Keyboard)
			}
		})
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
