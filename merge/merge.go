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
//
// Known gap (#6): interaction settings resolve by Go zero value rather than by field presence,
// so a higher-precedence layer can only switch an interaction on, never off, and a manifest's
// mouse block replaces the lower layer's wholesale instead of merging key by key. That predates
// the inlineManifest slot but is easier to reach now that two manifests stack.
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
// The error is the inline manifest's decode error, and only when ref is nil: with an
// authoritative manifest in hand the inline fallback goes unread, so a malformed one cannot
// block rendering.
func DisplayForItem(def *playlist.Defaults, ref *refmanifest.Manifest, item playlist.PlaylistItem) (*playlist.DisplayPrefs, error) {
	var base playlist.DisplayPrefs
	if def != nil && def.Display != nil {
		base = *cloneDisplay(def.Display)
	}
	// An undecodable inline manifest is fatal only when nothing can stand in for it. §3.6 makes
	// a fetched ref authoritative and the inline copy the fallback, so when ref is present the
	// fallback is simply not used — refusing to render because the copy nobody would have read
	// is malformed would be the wrong call.
	inline, err := item.ParseInlineManifest()
	if err != nil && ref == nil {
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
// and are layered by [DisplayForItem].
//
// The error is the inline manifest's decode error, reached only when ref is nil, since a
// non-nil ref is returned without reading the inline copy. It is the normal outcome on the
// core-only parse path, where nothing has checked the field.
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

// cloneDisplay copies the defaults into the merge base. It must be deep: the result is handed
// to the caller, and every pointer or slice left shared with the playlist defaults is one a
// player can write through to corrupt the baseline for every later item. The overlays
// themselves always allocate, so this is the only place that sharing could originate.
func cloneDisplay(d *playlist.DisplayPrefs) *playlist.DisplayPrefs {
	c := *d
	if d.Autoplay != nil {
		v := *d.Autoplay
		c.Autoplay = &v
	}
	if d.Loop != nil {
		v := *d.Loop
		c.Loop = &v
	}
	if d.Margin != nil {
		c.Margin = append(json.RawMessage(nil), d.Margin...)
	}
	if d.Interaction != nil {
		ip := *d.Interaction
		if d.Interaction.Mouse != nil {
			mp := *d.Interaction.Mouse
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
		if len(src.Interaction.Keyboard) > 0 {
			dst.Interaction.Keyboard = append([]string(nil), src.Interaction.Keyboard...)
		}
		if src.Interaction.Mouse != nil {
			if dst.Interaction.Mouse == nil {
				dst.Interaction.Mouse = &playlist.MousePrefs{}
			}
			m := dst.Interaction.Mouse
			sm := src.Interaction.Mouse
			if sm.Click {
				m.Click = sm.Click
			}
			if sm.Scroll {
				m.Scroll = sm.Scroll
			}
			if sm.Drag {
				m.Drag = sm.Drag
			}
			if sm.Hover {
				m.Hover = sm.Hover
			}
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
	if len(src.Interaction) > 0 {
		if dst.Interaction == nil {
			dst.Interaction = &playlist.InteractionPrefs{}
		}
		var ip playlist.InteractionPrefs
		if err := json.Unmarshal(src.Interaction, &ip); err == nil {
			if len(ip.Keyboard) > 0 {
				dst.Interaction.Keyboard = ip.Keyboard
			}
			if ip.Mouse != nil {
				dst.Interaction.Mouse = ip.Mouse
			}
		}
	}
}

func isEmptyDisplay(d playlist.DisplayPrefs) bool {
	return d.Scaling == "" && len(d.Margin) == 0 && d.Background == "" &&
		d.Autoplay == nil && d.Loop == nil && d.Interaction == nil && len(d.UserOverrides) == 0
}
