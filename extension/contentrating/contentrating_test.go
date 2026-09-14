package contentrating_test

import (
	"errors"
	"testing"

	dp1 "github.com/display-protocol/dp1-go"
	"github.com/display-protocol/dp1-go/extension/contentrating"
)

// ValidatePlaylistFragment is the extension-local entrypoint for callers that hold only the
// content-rating overlay (an ingestion boundary that cannot yet require a signed DP-1 document).
// The cases below lock the fragment contract: core fields and signatures are not required, an
// absent rating is unrated, and a present malformed rating or reason is rejected.
func TestValidatePlaylistFragment(t *testing.T) {
	t.Parallel()
	valid := []string{
		`{}`,
		`{"items":[]}`,
		`{"items":[{}]}`,
		`{"items":[{"contentRating":"general"}]}`,
		`{"items":[{"contentRating":"mature","contentReasons":["nudity"]}]}`,
		`{"items":[{"contentReasons":[]}]}`,
	}
	for _, raw := range valid {
		if err := contentrating.ValidatePlaylistFragment([]byte(raw)); err != nil {
			t.Fatalf("valid fragment %s: %v", raw, err)
		}
	}

	invalid := []string{
		`{"items":[{"contentRating":null}]}`,
		`{"items":[{"contentRating":"unrated"}]}`,
		`{"items":[{"contentRating":1}]}`,
		`{"items":[{"contentReasons":null}]}`,
		`{"items":[{"contentReasons":[""]}]}`,
		`{"items":[{"contentReasons":"nudity"}]}`,
	}
	for _, raw := range invalid {
		err := contentrating.ValidatePlaylistFragment([]byte(raw))
		if err == nil {
			t.Fatalf("expected invalid fragment: %s", raw)
		}
		if !errors.Is(err, dp1.ErrValidation) {
			t.Fatalf("fragment %s: want ErrValidation, got %v", raw, err)
		}
	}
}

// The extension-local helper and the root-package helper must stay the same check; they differ
// only in that the root wraps failures with a DP-1 error code for the UI/telemetry contract.
func TestValidatePlaylistFragmentMatchesRootEntrypoint(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{
		`{"items":[{"contentRating":"mature"}]}`,
		`{"items":[{"contentRating":"unrated"}]}`,
	} {
		local := contentrating.ValidatePlaylistFragment([]byte(raw)) != nil
		root := dp1.ValidateContentRatingExtension([]byte(raw)) != nil
		if local != root {
			t.Fatalf("fragment %s: local invalid=%v, root invalid=%v", raw, local, root)
		}
	}
}

func TestRatingConstants(t *testing.T) {
	t.Parallel()
	if contentrating.RatingGeneral != "general" || contentrating.RatingMature != "mature" {
		t.Fatalf("rating constants must match the schema enum: %q %q",
			contentrating.RatingGeneral, contentrating.RatingMature)
	}
}
