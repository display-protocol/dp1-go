// Package contentrating defines the draft DP-1 content-rating extension types.
package contentrating

import "github.com/display-protocol/dp1-go/internal/validate"

// Rating is a curatorial audience label.
//
// The wire vocabulary is open (DP-1 content-rating extension §3.3): any string is a valid
// rating, and v0.1.0 defines only "general" and "mature". Absence, or a value the consumer does
// not recognize, means unrated — nothing is assumed from an unknown label, so an unfamiliar
// rating never causes an item to be hidden. Only RatingMature hides anything.
//
// That is why this is a string type with constants rather than a closed enumeration: a rating
// from a later vocabulary decodes, round-trips, and re-encodes unchanged, which keeps the JCS
// payload intact for a consumer that merely passes the document along. Use Known to branch on
// the values this SDK version understands.
type Rating string

const (
	RatingGeneral Rating = "general"
	RatingMature  Rating = "mature"
)

// Known reports whether r is a rating this SDK version defines.
//
// A false result is not an error and not a reason to reject the item: per §3.3 an unrecognized
// rating is treated exactly as an absent one, i.e. unrated. Branch on Known only to decide
// whether a label can be acted on, never to decide whether a document is valid.
func (r Rating) Known() bool {
	switch r {
	case RatingGeneral, RatingMature:
		return true
	default:
		return false
	}
}

// ValidatePlaylistFragment validates only content-rating extension fields.
// It intentionally does not require core fields or signatures.
func ValidatePlaylistFragment(data []byte) error {
	return validate.ContentRatingExtensionFragment(data)
}
