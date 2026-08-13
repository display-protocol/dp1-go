// Package merge applies DP-1 resolution order for display (and related) fields on an item:
// defaults → item inlineManifest → ref manifest → item override (JSON) → item-local fields
// (last wins for the same path).
//
// The inlineManifest slot sits immediately below ref (playlists extension §3.6): a manifest
// fetched via ref is authoritative and the inline copy is the offline/degraded fallback.
//
// This package is not a validation boundary: it overlays whatever manifests it is handed.
// item.InlineManifest is only schema-checked when the playlist was parsed with the playlists
// extension (dp1.ParseAndValidatePlaylistWithPlaylistsExtension); the core-only parser
// tolerates it as an unknown field. A fetched ref manifest is likewise the caller's to
// validate (dp1.ParseAndValidateRefManifest). Overlay unvalidated manifests and values the
// schema would reject — a scaling outside the enum, say — reach the returned DisplayPrefs.
package merge

import (
	"encoding/json"

	"github.com/display-protocol/dp1-go/playlist"
	"github.com/display-protocol/dp1-go/refmanifest"
)

// DisplayForItem returns merged display preferences for an item.
// ref may be nil if no manifest was fetched; item.InlineManifest (if any) is applied
// underneath it, so callers do not pass the inline manifest separately.
func DisplayForItem(def *playlist.Defaults, ref *refmanifest.Manifest, item playlist.PlaylistItem) (*playlist.DisplayPrefs, error) {
	var base playlist.DisplayPrefs
	if def != nil && def.Display != nil {
		base = *cloneDisplay(def.Display)
	}
	applyManifestDisplay(&base, item.InlineManifest)
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
func ManifestForItem(ref *refmanifest.Manifest, item playlist.PlaylistItem) *refmanifest.Manifest {
	if ref != nil {
		return ref
	}
	return item.InlineManifest
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
