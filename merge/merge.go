// Package merge applies DP-1 resolution order for display (and related) fields on an item:
// defaults → item inlineManifest → ref manifest → item override (JSON) → item-local fields
// (last wins for the same path).
//
// The inlineManifest slot sits immediately below ref (playlists extension §3.6): a manifest
// fetched via ref is authoritative and the inline copy is the offline/degraded fallback.
//
// This package is not a validation boundary: it overlays whatever manifests it is handed, so
// values the schema would reject reach the result. item.InlineManifest is schema-checked only
// when the playlist was parsed with dp1.ParseAndValidatePlaylistWithPlaylistsExtension.
package merge

import (
	"encoding/json"

	"github.com/display-protocol/dp1-go/playlist"
	"github.com/display-protocol/dp1-go/refmanifest"
)

// DisplayForItem returns merged display preferences for an item.
// ref may be nil if no manifest was fetched; item.InlineManifest (if any) is applied
// underneath it, so callers do not pass the inline manifest separately.
//
// Note this resolves per key while [ManifestForItem] resolves per document: a key the fetched
// ref manifest leaves unset still comes from the inline copy here, even though ManifestForItem
// would have discarded that copy wholesale. That follows the spec — ref-manifest §7 is
// explicitly last-write-wins within the same key path — but the two functions can disagree
// about where a given value came from.
func DisplayForItem(def *playlist.Defaults, ref *refmanifest.Manifest, item playlist.PlaylistItem) (*playlist.DisplayPrefs, error) {
	var base playlist.DisplayPrefs
	if def != nil && def.Display != nil {
		base = *cloneDisplay(def.Display)
	}
	inline, err := item.ParseInlineManifest()
	if err != nil {
		return nil, err
	}
	applyManifestDisplay(&base, inline)
	applyManifestDisplay(&base, ref)
	if len(item.Override) > 0 {
		var ov struct {
			Duration *float64               `json:"duration,omitempty"`
			Display  *playlist.DisplayPrefs `json:"display,omitempty"`
		}
		if err := json.Unmarshal(item.Override, &ov); err != nil {
			return nil, err
		}
		if ov.Display != nil {
			overlayDisplay(&base, ov.Display)
		}
	}
	if item.Display != nil {
		overlayDisplay(&base, item.Display)
	}
	if isEmptyDisplay(base) {
		return nil, nil
	}
	return &base, nil
}

// ManifestForItem returns the ref manifest a player should read for an item, applying the
// §3.6 precedence: a manifest fetched via ref wins over the item's inlineManifest, which in
// turn serves offline or degraded-fetch paths. Pass a nil ref when no fetch was made (or it
// failed) to fall back to the inline copy. Returns nil when neither is present.
//
// This resolves the whole document, not per-field: the two manifests are alternative carriages
// of one document (§3.6), so they are not merged key by key. Display controls are the exception
// and are layered by [DisplayForItem]. The error is the inline manifest's decode error; it
// cannot occur for a playlist parsed with dp1.ParseAndValidatePlaylistWithPlaylistsExtension.
func ManifestForItem(ref *refmanifest.Manifest, item playlist.PlaylistItem) (*refmanifest.Manifest, error) {
	if ref != nil {
		return ref, nil
	}
	return item.ParseInlineManifest()
}

// applyManifestDisplay overlays one manifest's display controls, tolerating a nil manifest or
// a manifest without controls so callers can chain inline and fetched manifests in order.
func applyManifestDisplay(dst *playlist.DisplayPrefs, m *refmanifest.Manifest) {
	if m == nil || m.Controls == nil || m.Controls.Display == nil {
		return
	}
	applyDisplayJSON(dst, m.Controls.Display)
}

func cloneDisplay(d *playlist.DisplayPrefs) *playlist.DisplayPrefs {
	c := *d
	if d.Interaction != nil {
		ip := *d.Interaction
		if d.Interaction.Keyboard != nil {
			ip.Keyboard = append([]string(nil), d.Interaction.Keyboard...)
		}
		if d.Interaction.Mouse != nil {
			// Copying the struct would alias the caller's *bool fields, so a later overlay
			// writing through them would reach back into the playlist defaults.
			mp := playlist.MousePrefs{}
			overlayBool(&mp.Click, d.Interaction.Mouse.Click)
			overlayBool(&mp.Scroll, d.Interaction.Mouse.Scroll)
			overlayBool(&mp.Drag, d.Interaction.Mouse.Drag)
			overlayBool(&mp.Hover, d.Interaction.Mouse.Hover)
			ip.Mouse = &mp
		}
		c.Interaction = &ip
	}
	if d.UserOverrides != nil {
		m := make(map[string]bool, len(d.UserOverrides))
		for k, v := range d.UserOverrides {
			m[k] = v
		}
		c.UserOverrides = m
	}
	return &c
}

// overlayBool writes src over dst only when src carries a value, copying it so layers never
// alias one another's pointers. A nil src leaves the lower layer's decision in place — the
// distinction that lets an item turn an interaction off rather than only on.
func overlayBool(dst **bool, src *bool) {
	if src == nil {
		return
	}
	v := *src
	*dst = &v
}

func overlayDisplay(dst *playlist.DisplayPrefs, src *playlist.DisplayPrefs) {
	if src.Scaling != "" {
		dst.Scaling = src.Scaling
	}
	if len(src.Margin) > 0 {
		dst.Margin = append(json.RawMessage(nil), src.Margin...)
	}
	if src.Background != "" {
		dst.Background = src.Background
	}
	if src.Autoplay != nil {
		v := *src.Autoplay
		dst.Autoplay = &v
	}
	if src.Loop != nil {
		v := *src.Loop
		dst.Loop = &v
	}
	if src.Interaction != nil {
		if dst.Interaction == nil {
			dst.Interaction = &playlist.InteractionPrefs{}
		}
		// Presence, not emptiness: an explicit "keyboard": [] revokes the keys a lower layer
		// allowed, so only a nil slice means "said nothing".
		if src.Interaction.Keyboard != nil {
			dst.Interaction.Keyboard = append([]string(nil), src.Interaction.Keyboard...)
		}
		if src.Interaction.Mouse != nil {
			if dst.Interaction.Mouse == nil {
				dst.Interaction.Mouse = &playlist.MousePrefs{}
			}
			m := dst.Interaction.Mouse
			sm := src.Interaction.Mouse
			overlayBool(&m.Click, sm.Click)
			overlayBool(&m.Scroll, sm.Scroll)
			overlayBool(&m.Drag, sm.Drag)
			overlayBool(&m.Hover, sm.Hover)
		}
	}
	if len(src.UserOverrides) > 0 {
		if dst.UserOverrides == nil {
			dst.UserOverrides = make(map[string]bool)
		}
		for k, v := range src.UserOverrides {
			dst.UserOverrides[k] = v
		}
	}
}

func applyDisplayJSON(dst *playlist.DisplayPrefs, src *refmanifest.DisplayControls) {
	if src.Scaling != "" {
		dst.Scaling = src.Scaling
	}
	if len(src.Margin) > 0 {
		dst.Margin = append(json.RawMessage(nil), src.Margin...)
	}
	if src.Background != "" {
		dst.Background = src.Background
	}
	if src.Autoplay != nil {
		v := *src.Autoplay
		dst.Autoplay = &v
	}
	if src.Loop != nil {
		v := *src.Loop
		dst.Loop = &v
	}
	applyInteractionJSON(dst, src.Interaction)
}

// applyInteractionJSON overlays one manifest's interaction block by JSON field presence, not by
// Go zero values. Two manifests now stack (inline then ref, §3.6), so assigning a decoded block
// wholesale would let a ref manifest setting only mouse.scroll erase a mouse.click the inline
// copy set — clobbering a key the higher-precedence document never mentioned, which
// ref-manifest §7 ("last-write-wins within the same key path") does not license.
//
// Presence is what the pointers below record: nil means the key was absent and the lower layer
// stands; non-nil means it was written and wins, including "keyboard": [] and "click": false,
// which a length or truthiness check would silently drop.
func applyInteractionJSON(dst *playlist.DisplayPrefs, raw json.RawMessage) {
	if len(raw) == 0 {
		return
	}
	var src struct {
		Keyboard *[]string `json:"keyboard"`
		Mouse    *struct {
			Click  *bool `json:"click"`
			Scroll *bool `json:"scroll"`
			Drag   *bool `json:"drag"`
			Hover  *bool `json:"hover"`
		} `json:"mouse"`
	}
	// A manifest that reached here was schema-checked by the caller's parser; a decode failure
	// means the interaction block is unusable, and dropping it leaves lower layers intact.
	if err := json.Unmarshal(raw, &src); err != nil {
		return
	}
	if src.Keyboard == nil && src.Mouse == nil {
		return
	}
	if dst.Interaction == nil {
		dst.Interaction = &playlist.InteractionPrefs{}
	}
	if src.Keyboard != nil {
		dst.Interaction.Keyboard = append([]string(nil), *src.Keyboard...)
	}
	if src.Mouse != nil {
		if dst.Interaction.Mouse == nil {
			dst.Interaction.Mouse = &playlist.MousePrefs{}
		}
		m := dst.Interaction.Mouse
		overlayBool(&m.Click, src.Mouse.Click)
		overlayBool(&m.Scroll, src.Mouse.Scroll)
		overlayBool(&m.Drag, src.Mouse.Drag)
		overlayBool(&m.Hover, src.Mouse.Hover)
	}
}

func isEmptyDisplay(d playlist.DisplayPrefs) bool {
	return d.Scaling == "" && len(d.Margin) == 0 && d.Background == "" &&
		d.Autoplay == nil && d.Loop == nil && d.Interaction == nil && len(d.UserOverrides) == 0
}
