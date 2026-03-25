// Package merge applies DP-1 resolution order for display (and related) fields on an item:
// defaults → ref manifest → item override (JSON) → item-local fields (last wins for the same path).
package merge

import (
	"encoding/json"

	"github.com/display-protocol/dp1-go/playlist"
	"github.com/display-protocol/dp1-go/refmanifest"
)

// DisplayForItem returns merged display preferences for an item.
// ref may be nil if no manifest was fetched.
func DisplayForItem(def *playlist.Defaults, ref *refmanifest.Manifest, item playlist.PlaylistItem) (*playlist.DisplayPrefs, error) {
	var base playlist.DisplayPrefs
	if def != nil && def.Display != nil {
		base = *cloneDisplay(def.Display)
	}
	if ref != nil && ref.Controls != nil && ref.Controls.Display != nil {
		applyDisplayJSON(&base, ref.Controls.Display)
	}
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
