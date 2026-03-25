// Package dp1 implements parsing, validation, and helpers for the DP-1 protocol
// (playlists, playlist-groups, ref manifests, and registered extensions).
//
// Entrypoints use JSON Schema (draft 2020-12) embedded from the specification:
// use ParseAndValidate* functions to obtain typed values that are known to match the schema.
package dp1
