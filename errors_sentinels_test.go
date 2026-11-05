package dp1

import (
	"errors"
	"testing"

	"github.com/display-protocol/dp1-go/internal/validate"
	"github.com/display-protocol/dp1-go/sign"
)

// TestSentinelsAreTheSameValues pins the re-exports in errors.go to the sentinels they alias.
// They exist so a consumer importing only this package can classify any error the SDK returns, so
// an alias that drifted — or a new sign sentinel left unmirrored, as happened to
// ErrPayloadHashMismatch during review of its own branch — silently costs that consumer a second
// import. Go cannot enumerate another package's variables at run time, so this list is maintained
// by hand: when the sign package gains a sentinel, add it here and to errors.go in the same change.
func TestSentinelsAreTheSameValues(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		exported error
		upstream error
	}{
		{"ErrValidation", ErrValidation, validate.ErrValidation},
		{"ErrSigInvalid", ErrSigInvalid, sign.ErrSigInvalid},
		{"ErrUnsupportedAlg", ErrUnsupportedAlg, sign.ErrUnsupportedAlg},
		{"ErrNoSignatures", ErrNoSignatures, sign.ErrNoSignatures},
		{"ErrPayloadHashMismatch", ErrPayloadHashMismatch, sign.ErrPayloadHashMismatch},
	}
	for _, c := range cases {
		// Both directions. What a consumer writes is the first: errors.Is against the
		// re-export, over an error the sign package produced. The second makes the pair
		// mutually identical rather than merely compatible — neither sentinel wraps anything,
		// so errors.Is both ways is the same statement as pointer equality, without tripping
		// errorlint's ban on comparing errors with !=.
		if !errors.Is(c.upstream, c.exported) {
			t.Errorf("errors.Is(sign/validate sentinel, dp1.%s) = false; the alias points elsewhere", c.name)
		}
		if !errors.Is(c.exported, c.upstream) {
			t.Errorf("errors.Is(dp1.%s, sign/validate sentinel) = false; the alias points elsewhere", c.name)
		}
	}
}
