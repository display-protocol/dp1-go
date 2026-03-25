package dp1

import (
	"errors"

	"github.com/display-protocol/dp1-go/internal/validate"
	"github.com/display-protocol/dp1-go/sign"
)

// Sentinel errors re-exported so applications can use a single import path with
// errors.Is / errors.As (e.g. signature verification vs JSON Schema validation).
var (
	ErrValidation     = validate.ErrValidation
	ErrSigInvalid     = sign.ErrSigInvalid
	ErrUnsupportedAlg = sign.ErrUnsupportedAlg
)

// ErrorCode is a stable string for DP-1 §14 (player → UI) mapping where applicable,
// plus SDK-specific codes for document types not named in §14.
type ErrorCode string

const (
	// CodePlaylistInvalid is the §14 code for playlist schema / structural failure (“Playlist malformed.”).
	// Use with ParseAndValidatePlaylist and ParseAndValidatePlaylistWithPlaylistsExtension only.
	CodePlaylistInvalid ErrorCode = "playlistInvalid"
	// CodePlaylistGroupInvalid is used when a playlist-group (exhibition) document fails JSON Schema validation or decode.
	CodePlaylistGroupInvalid ErrorCode = "playlistGroupInvalid"
	// CodeRefManifestInvalid is used when a ref manifest document fails JSON Schema validation or decode.
	CodeRefManifestInvalid ErrorCode = "refManifestInvalid"
	// CodeChannelInvalid is used when a channel extension document fails JSON Schema validation or decode.
	CodeChannelInvalid ErrorCode = "channelInvalid"
	// CodeSigInvalid is the §14 code for signature verification failure (“Invalid feed signature.”).
	// Returned by the sign package; not emitted from ParseAndValidate* helpers.
	CodeSigInvalid ErrorCode = "sigInvalid"
	// The following §14 codes are reserved for player/runtime behavior (auth, hash drift, network).
	// This parsing/signing SDK does not emit them; clients may use them when implementing a full player.
	CodeLicenseDenied     ErrorCode = "licenseDenied"
	CodeReproMismatch     ErrorCode = "reproMismatch"
	CodeSourceUnreachable ErrorCode = "sourceUnreachable"
)

// CodedError wraps an error with ErrorCode for UI or telemetry.
type CodedError struct {
	Code ErrorCode
	Err  error
}

func (e *CodedError) Error() string { return string(e.Code) + ": " + e.Err.Error() }
func (e *CodedError) Unwrap() error { return e.Err }

// WithCode wraps err when err is non-nil.
func WithCode(code ErrorCode, err error) error {
	if err == nil {
		return nil
	}
	return &CodedError{Code: code, Err: err}
}

func validationErr(err error, code ErrorCode) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, validate.ErrValidation) {
		return WithCode(code, err)
	}
	return err
}

// CodeFromPlaylistValidation maps JSON Schema validation failures to CodePlaylistInvalid
// (playlist core or playlist + playlists extension schemas).
// The wrapped error still satisfies errors.Is(err, ErrValidation) for the underlying failure.
func CodeFromPlaylistValidation(err error) error {
	return validationErr(err, CodePlaylistInvalid)
}

// CodeFromPlaylistGroupValidation maps JSON Schema validation failures to CodePlaylistGroupInvalid.
func CodeFromPlaylistGroupValidation(err error) error {
	return validationErr(err, CodePlaylistGroupInvalid)
}

// CodeFromRefManifestValidation maps JSON Schema validation failures to CodeRefManifestInvalid.
func CodeFromRefManifestValidation(err error) error {
	return validationErr(err, CodeRefManifestInvalid)
}

// CodeFromChannelValidation maps JSON Schema validation failures to CodeChannelInvalid.
func CodeFromChannelValidation(err error) error {
	return validationErr(err, CodeChannelInvalid)
}
