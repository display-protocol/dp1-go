package sign

import (
	"crypto/ed25519"
	"fmt"
	"strings"

	mb "github.com/multiformats/go-multibase"
)

const didKeyPrefix = "did:key:"

// Ed25519DIDKey returns the did:key identifier for a raw Ed25519 public key (32 bytes),
// using the W3C did:key method: multicodec-prefixed key material and multibase base58btc.
//
// References: https://www.w3.org/TR/did-core/ , https://w3c-ccg.github.io/did-key-spec/
func Ed25519DIDKey(pub ed25519.PublicKey) (string, error) {
	if len(pub) != ed25519.PublicKeySize {
		return "", fmt.Errorf("ed25519 public key must be %d bytes, got %d", ed25519.PublicKeySize, len(pub))
	}
	// ed25519-pub multicodec (varint 0xed, 0x01); see multiformats/multicodec.
	desc := make([]byte, 0, 2+ed25519.PublicKeySize)
	desc = append(desc, 0xed, 0x01)
	desc = append(desc, pub...)
	enc, err := mb.Encode(mb.Base58BTC, desc)
	if err != nil {
		return "", err
	}
	return didKeyPrefix + enc, nil
}

// Ed25519PublicKeyFromDIDKey parses kid as a W3C did:key for an Ed25519 public key: multibase
// base58btc (leading z) over ed25519-pub multicodec (0xed, 0x01) plus 32 raw key bytes.
// The did:key: prefix is matched case-insensitively; the multibase substring keeps its original casing.
//
// Returned errors describe why parsing failed
func Ed25519PublicKeyFromDIDKey(kid string) (ed25519.PublicKey, error) {
	if len(kid) < len(didKeyPrefix) || !strings.EqualFold(kid[:len(didKeyPrefix)], didKeyPrefix) {
		return nil, fmt.Errorf("kid must use did:key form")
	}
	mbStr := kid[len(didKeyPrefix):]
	if mbStr == "" {
		return nil, fmt.Errorf("empty did:key method-specific id")
	}
	enc, data, err := mb.Decode(mbStr)
	if err != nil {
		return nil, fmt.Errorf("decode did:key multibase: %w", err)
	}
	if enc != mb.Base58BTC {
		return nil, fmt.Errorf("did:key must use multibase base58btc")
	}
	if len(data) != 2+ed25519.PublicKeySize {
		return nil, fmt.Errorf("did:key key material length: want %d bytes, got %d", 2+ed25519.PublicKeySize, len(data))
	}
	if data[0] != 0xed || data[1] != 0x01 {
		return nil, fmt.Errorf("did:key is not ed25519-pub multicodec")
	}
	return ed25519.PublicKey(data[2:]), nil
}
