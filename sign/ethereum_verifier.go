package sign

import (
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/display-protocol/dp1-go/playlist"
)

// EthereumVerifier implements signature verification for the eip191 algorithm.
// It uses did:pkh identifiers (CAIP-10 format with eip155 namespace) and verifies
// ECDSA secp256k1 signatures over the DP-1 signing digest using Ethereum's personal_sign
// message format (EIP-191 version 0x45).
//
// Replay Attack Considerations:
//   - Cross-document replay: PREVENTED by payload_hash validation
//   - Cross-chain replay: NOT PREVENTED. The same signature is valid across all EVM
//     chains for a given address. Applications requiring explicit chain-binding should
//     validate the chainId from the kid field.
//   - Temporal replay: NOT PREVENTED. Applications should validate timestamp requirements.
type EthereumVerifier struct{}

// Alg returns "eip191" to match playlist.AlgEIP191.
func (v *EthereumVerifier) Alg() string {
	return playlist.AlgEIP191
}

// VerifySignature verifies an Ethereum personal_sign (EIP-191) signature.
//
// The verification process:
//  1. Parse kid as did:pkh:eip155:{chainID}:{address} to extract the signer's address
//  2. Validate signature format (65 bytes: r[32] + s[32] + v[1])
//  3. Normalize v value (27/28 -> 0/1) for recovery
//  4. Recover public key from signature and digest
//  5. Derive address from recovered public key
//  6. Compare recovered address with kid address (case-insensitive)
//
// Note: The digest parameter is the DP-1 signing hash (32 bytes). This is signed directly
// in Ethereum personal_sign format without additional prefixing at this layer, as the
// prefixing ("\x19Ethereum Signed Message:\n32" + digest) is handled by the wallet/signing
// library during signature creation.
//
// Returns ErrSigInvalid if signature verification fails, or another error if kid parsing
// or signature format validation fails.
func (v *EthereumVerifier) VerifySignature(kid string, sigBytes []byte, digest [32]byte) error {
	// Extract address from did:pkh
	addr, _, err := EthereumAddressFromDIDPKH(kid)
	if err != nil {
		return fmt.Errorf("parse did:pkh: %w", err)
	}

	// Validate signature length (ECDSA signature: r[32] + s[32] + v[1])
	if len(sigBytes) != 65 {
		return fmt.Errorf("%w: ethereum signature must be 65 bytes, got %d", ErrSigInvalid, len(sigBytes))
	}

	// Normalize v value: some implementations use 27/28, others use 0/1
	// The crypto.Ecrecover function expects 0/1
	vByte := sigBytes[64]
	if vByte >= 27 {
		sigBytes = append([]byte(nil), sigBytes...) // Copy to avoid mutation
		sigBytes[64] -= 27
	}

	// Recover public key from signature
	// Apply Ethereum personal_sign message prefix: "\x19Ethereum Signed Message:\n32" + digest
	// This matches what wallets do when signing with personal_sign / eth_sign
	message := accounts.TextHash(digest[:])
	pubKey, err := crypto.SigToPub(message, sigBytes)
	if err != nil {
		return fmt.Errorf("%w: recover public key: %w", ErrSigInvalid, err)
	}

	// Derive address from recovered public key
	recoveredAddr := crypto.PubkeyToAddress(*pubKey)

	// Compare addresses (case-insensitive, as EIP-55 checksum may vary)
	if !strings.EqualFold(recoveredAddr.Hex(), addr) {
		return fmt.Errorf("%w: address mismatch: recovered %s, expected %s", ErrSigInvalid, recoveredAddr.Hex(), addr)
	}

	return nil
}

func init() {
	RegisterVerifier(&EthereumVerifier{})
}
