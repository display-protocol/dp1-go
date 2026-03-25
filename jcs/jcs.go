// Package jcs exposes RFC 8785 JSON Canonicalization for DP-1 signing payloads.
package jcs

import (
	gwjcs "github.com/gowebpki/jcs"
)

// Transform returns the canonical UTF-8 JSON form of the input document.
func Transform(json []byte) ([]byte, error) {
	return gwjcs.Transform(json)
}
