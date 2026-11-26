// Package identity holds shared types used by multiple DP-1 registry extensions.
package identity

import (
	"encoding/json"

	"github.com/display-protocol/dp1-go/internal/jsonint"
)

// Entity is a person or organization named by a DP-1 document: an artist credited on a
// work, a curator or partner of a playlist or channel, a channel publisher, or the author
// of a note. One model for every place DP-1 names someone (core ref-manifest schema
// $defs/Entity). Beyond Name and Key, the fields are a profile snapshot taken when the
// document was authored; consumers MAY overlay fresher registry data keyed by Addresses
// or Key. Role/Location/Addresses/Avatar/Biographies/Links were added alongside dp1#55's
// playlist-level OpenAt — both landed in the same upstream spec change, and this SDK had
// fallen behind the spec's own Entity consolidation (which had already folded the older,
// artist-only "Artist" shape into this one) even before #55.
type Entity struct {
	Name string `json:"name"`
	Key  string `json:"key,omitempty"`
	// ID is a unique, producer-scoped identifier for the entity — distinct from Key, which
	// is a verifiable DID. Not every producer has a DID for every entity it names.
	ID  string `json:"id,omitempty"`
	URL string `json:"url,omitempty"` // Deprecated: superseded by Links (write a "website" or "profile" entry).
	// Role disambiguates an entity's function when its position in the document does not
	// already say it (omitted, the role follows the position: metadata.artists[] is
	// artist, curators[] is curator, publisher is publisher).
	Role        string      `json:"role,omitempty"`
	Location    string      `json:"location,omitempty"`
	Addresses   []string    `json:"addresses,omitempty"`
	Avatar      *Thumbnail  `json:"avatar,omitempty"`
	Biographies []Biography `json:"biographies,omitempty"`
	Links       []Link      `json:"links,omitempty"`
}

// Thumbnail is an image reference shared across every DP-1 place that names one (an
// entity's avatar, a link's representative image, a playlist's cover).
type Thumbnail struct {
	URI    string `json:"uri"`
	W      *int   `json:"w,omitempty"`
	H      *int   `json:"h,omitempty"`
	SHA256 string `json:"sha256,omitempty"`
}

// UnmarshalJSON accepts every spelling of w and h the schema's integer accepts (100,
// 100.0, 1e2). Absent, null and unrelated members behave as with the default decoder.
//
// Embedding Thumbnail anonymously in another struct promotes this method and would skip
// the outer struct's own fields; use a named field instead.
func (t *Thumbnail) UnmarshalJSON(data []byte) error {
	type plain Thumbnail // strips this method to avoid recursion
	shadow := struct {
		*plain
		W *jsonint.Int `json:"w,omitempty"`
		H *jsonint.Int `json:"h,omitempty"`
	}{plain: (*plain)(t), W: (*jsonint.Int)(t.W), H: (*jsonint.Int)(t.H)}
	if err := json.Unmarshal(data, &shadow); err != nil {
		return err
	}
	t.W, t.H = (*int)(shadow.W), (*int)(shadow.H)
	return nil
}

// Biography is one biographical text with optional attribution. An Entity may carry
// several, in the producer's order of preference; the first is the one to show when only
// one fits.
type Biography struct {
	Text      string `json:"text"`
	Source    string `json:"source,omitempty"`
	SourceURL string `json:"sourceUrl,omitempty"`
}

// Link types accepted by the schema's links[].type enumeration (core ref-manifest
// $defs/Link). A destination the enumeration does not name is written as LinkTypeOther.
const (
	LinkTypeWebsite     = "website"
	LinkTypeTwitter     = "twitter"
	LinkTypeInstagram   = "instagram"
	LinkTypeProfile     = "profile"
	LinkTypePublication = "publication"
	LinkTypeArticle     = "article"
	LinkTypeEvent       = "event"
	LinkTypeNews        = "news"
	LinkTypeOther       = "other"
)

// Link is an external link, from an entity's profile (website, publication) or from a
// playlist (editorial article, event, news). Title/Date/Thumbnail/Authors are dp1#55
// additions on top of the original Type+URL-only shape, for the playlist-level editorial
// use (an article needs a headline and a byline; a profile link usually needs neither).
type Link struct {
	Type string `json:"type"`
	URL  string `json:"url"`
	// Title is a human-readable label for the destination, such as a publication or
	// article title.
	Title string `json:"title,omitempty"`
	// Date is an RFC3339 timestamp the linked content refers to (an article's publish
	// date, an event's start), not when this Link entry itself was written.
	Date      string     `json:"date,omitempty"`
	Thumbnail *Thumbnail `json:"thumbnail,omitempty"`
	// Authors credits who wrote or presented the linked content, the way
	// metadata.artists[] credits a work. A producer holding one joined credit line emits
	// a single entry carrying it, as it would for a collaboration.
	Authors []Entity `json:"authors,omitempty"`
}
