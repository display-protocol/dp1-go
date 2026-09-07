// Package contentrating defines the draft DP-1 content-rating extension types.
package contentrating

// Rating is a curatorial audience label. Its absence on a playlist item means unrated.
type Rating string

const (
	RatingGeneral Rating = "general"
	RatingMature  Rating = "mature"
)
