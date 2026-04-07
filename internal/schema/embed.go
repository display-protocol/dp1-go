package schema

import "embed"

// FS holds embedded copies of DP-1 JSON Schemas from the specification repository.
//
//go:embed all:core
//go:embed all:extensions
var FS embed.FS
