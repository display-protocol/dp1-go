package validate

import "errors"

// ErrValidation indicates JSON Schema validation failed.
var ErrValidation = errors.New("dp1: validation failed")
