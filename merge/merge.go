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

// DurationForItem resolves an item's duration in seconds: the item's own duration, else
// override.duration, else defaults.duration (§3.1 inheritance, §3.2 resolution order). Nil
// when none is set, which leaves the choice to the player.
//
// override.duration sits between the item field and the defaults for the same reason
// override.display does in [DisplayForItem]. A malformed override is an error rather than
// being skipped, matching DisplayForItem.
func DurationForItem(def *playlist.Defaults, item playlist.PlaylistItem) (*float64, error) {
	if item.Duration != nil {
		v := *item.Duration
		return &v, nil
	}
	if len(item.Override) > 0 {
		// Exact-key lookup: a struct decode would match an unknown "Duration" member
		// case-insensitively and let it replace or supply the recognized "duration".
		var ov map[string]json.RawMessage
		if err := json.Unmarshal(item.Override, &ov); err != nil {
			return nil, err
		}
		if raw, ok := ov["duration"]; ok {
			var d *float64
			if err := json.Unmarshal(raw, &d); err != nil {
				return nil, err
			}
			if d != nil {
				return d, nil
			}
		}
	}
	if def != nil && def.Duration != nil {
		v := *def.Duration
		return &v, nil
	}
	return nil, nil
}

// LicenseForItem resolves an item's license mode: the item's own value, else the playlist
// defaults' (§3.1 inheritance rule). Empty when neither sets one.
func LicenseForItem(def *playlist.Defaults, item playlist.PlaylistItem) string {
	if item.License != "" {
		return item.License
	}
	if def != nil {
		return def.License
	}
	return ""
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

func cloneDisplay(d *playlist.DisplayPrefs) *playlist.DisplayPrefs {
	c := *d
	if d.Interaction != nil {
		ip := *d.Interaction
		if d.Interaction.Keyboard != nil {
			ip.Keyboard = append([]string{}, d.Interaction.Keyboard...)
		}
		if d.Interaction.Mouse != nil {
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
	if src.ImageRendering != "" {
		dst.ImageRendering = src.ImageRendering
	}
	overlayInteraction(dst, src.Interaction)
	if len(src.UserOverrides) > 0 {
		if dst.UserOverrides == nil {
			dst.UserOverrides = make(map[string]bool)
		}
		for k, v := range src.UserOverrides {
			dst.UserOverrides[k] = v
		}
	}
}

// applyDisplayJSON overlays a ref manifest's display controls. Every DisplayControls field
// needs a case here: one without a case decodes and is then ignored, so the manifest's
// value never reaches the player.
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
	if src.ImageRendering != "" {
		dst.ImageRendering = src.ImageRendering
	}
	if len(src.Interaction) > 0 {
		if dst.Interaction == nil {
			dst.Interaction = &playlist.InteractionPrefs{}
		}
		var ip playlist.InteractionPrefs
		// A manifest whose interaction block does not decode is skipped, as before: the schema
		// path rejects it, and the merge package is not a validation boundary.
		if err := json.Unmarshal(src.Interaction, &ip); err == nil {
			overlayInteraction(dst, &ip)
		}
	}
}

// overlayInteraction layers one interaction block over dst by field presence, so a higher
// layer can switch an interaction off as well as on and a layer that names only some keys
// leaves the rest as the lower layer set them (§4, last write wins per key path). An explicit
// empty keyboard list ("keyboard": []) is a revocation, hence the nil check rather than len.
func overlayInteraction(dst *playlist.DisplayPrefs, src *playlist.InteractionPrefs) {
	if src == nil {
		return
	}
	if dst.Interaction == nil {
		dst.Interaction = &playlist.InteractionPrefs{}
	}
	if src.Keyboard != nil {
		dst.Interaction.Keyboard = append([]string{}, src.Keyboard...)
	}
	if src.Mouse == nil {
		return
	}
	if dst.Interaction.Mouse == nil {
		dst.Interaction.Mouse = &playlist.MousePrefs{}
	}
	m, sm := dst.Interaction.Mouse, src.Mouse
	overlayBool(&m.Click, sm.Click)
	overlayBool(&m.Scroll, sm.Scroll)
	overlayBool(&m.Drag, sm.Drag)
	overlayBool(&m.Hover, sm.Hover)
}

func overlayBool(dst **bool, src *bool) {
	if src != nil {
		v := *src
		*dst = &v
	}
}

func isEmptyDisplay(d playlist.DisplayPrefs) bool {
	return d.Scaling == "" && len(d.Margin) == 0 && d.Background == "" &&
		d.Autoplay == nil && d.Loop == nil && d.ImageRendering == "" &&
		d.Interaction == nil && len(d.UserOverrides) == 0
}
