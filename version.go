package dp1

import (
	"fmt"

	"github.com/Masterminds/semver/v3"
)

// ParseDPVersion parses a document dpVersion field (SemVer), per DP-1 §12.
// Typical use: after decoding a playlist, compare or log against the player’s supported spec version.
func ParseDPVersion(s string) (*semver.Version, error) {
	return semver.NewVersion(s)
}

// WarnMajorMismatch is an optional policy helper for §12 (“players warn on major mismatch”).
// Pass the parsed dpVersion and the major version your player implements (e.g. 1 for DP-1 v1.x).
// Nil document is a no-op so callers can skip parsing when the field is absent.
func WarnMajorMismatch(document *semver.Version, wantMajor uint64) error {
	if document == nil {
		return nil
	}
	if document.Major() != wantMajor {
		return fmt.Errorf("dp1: major version mismatch: document %s, want major %d", document, wantMajor)
	}
	return nil
}
