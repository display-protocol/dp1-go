// Package contentrating defines the draft DP-1 content-rating extension types.
package contentrating

import "github.com/display-protocol/dp1-go/internal/validate"

// Rating is a curatorial audience label. Its absence on a playlist item means unrated.
type Rating string

const (
	RatingGeneral Rating = "general"
	RatingMature  Rating = "mature"
)

// ValidatePlaylistFragment validates only content-rating extension fields.
// It intentionally does not require core fields or signatures.
func ValidatePlaylistFragment(data []byte) error {
	return validate.ContentRatingExtensionFragment(data)
}
