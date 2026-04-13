package sign

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/common"
)

const didPKHPrefix = "did:pkh:"

// EthereumAddressToDIDPKH converts an Ethereum address to did:pkh format following CAIP-10.
//
// Format: did:pkh:eip155:{chainID}:{address}
//
// The address is normalized to EIP-55 mixed-case checksum format. The chainID identifies
// the EVM-compatible chain (1 = Ethereum mainnet, 137 = Polygon, 42161 = Arbitrum, etc.).
//
// Examples:
//   - Ethereum mainnet: did:pkh:eip155:1:0xb9c5714089478a327f09197987f16f9e5d936e8a
//   - Polygon: did:pkh:eip155:137:0xb9c5714089478a327f09197987f16f9e5d936e8a
//   - Arbitrum: did:pkh:eip155:42161:0xb9c5714089478a327f09197987f16f9e5d936e8a
//
// See: https://github.com/w3c-ccg/did-pkh/blob/main/did-pkh-method-draft.md
// See: https://chainagnostic.org/CAIPs/caip-10
func EthereumAddressToDIDPKH(addr string, chainID int) (string, error) {
	if chainID < 1 {
		return "", fmt.Errorf("chainID must be positive, got %d", chainID)
	}
	if !common.IsHexAddress(addr) {
		return "", fmt.Errorf("invalid ethereum address format: %q", addr)
	}
	// Normalize to EIP-55 checksum (go-ethereum's common.Address handles this)
	checksummed := common.HexToAddress(addr).Hex()
	return fmt.Sprintf("%seip155:%d:%s", didPKHPrefix, chainID, checksummed), nil
}

// EthereumAddressFromDIDPKH extracts and validates an Ethereum address from a did:pkh identifier.
//
// Expected format: did:pkh:eip155:{chainID}:{address}
//
// Returns:
//   - address: EIP-55 checksummed address (0x-prefixed, 42 characters)
//   - chainID: The EVM chain ID
//   - error: Non-nil if the DID format is invalid or not an Ethereum did:pkh
//
// The function validates:
//   - Correct did:pkh prefix (case-insensitive)
//   - eip155 namespace (Ethereum/EVM chains)
//   - Valid chainID (positive integer)
//   - Valid Ethereum address format
//   - EIP-55 checksum correctness
func EthereumAddressFromDIDPKH(kid string) (string, int, error) {
	if len(kid) < len(didPKHPrefix) || !strings.EqualFold(kid[:len(didPKHPrefix)], didPKHPrefix) {
		return "", 0, fmt.Errorf("kid must use did:pkh form, got %q", kid)
	}
	methodSpecificID := kid[len(didPKHPrefix):]
	parts := strings.Split(methodSpecificID, ":")
	if len(parts) != 3 {
		return "", 0, fmt.Errorf("invalid did:pkh format: expected 3 colon-separated parts after 'did:pkh:', got %d", len(parts))
	}
	namespace := parts[0]
	chainIDStr := parts[1]
	addr := parts[2]

	if namespace != "eip155" {
		return "", 0, fmt.Errorf("unsupported did:pkh namespace %q; expected 'eip155' for Ethereum", namespace)
	}

	chainID, err := strconv.Atoi(chainIDStr)
	if err != nil || chainID < 1 {
		return "", 0, fmt.Errorf("invalid chainID in did:pkh: %q", chainIDStr)
	}

	if !common.IsHexAddress(addr) {
		return "", 0, fmt.Errorf("invalid ethereum address in did:pkh: %q", addr)
	}

	// Validate and normalize to EIP-55 checksum
	parsed := common.HexToAddress(addr)
	checksummed := parsed.Hex()

	// Verify the original address matches the checksum (case-sensitive)
	// If input was not checksummed or had wrong case, this catches it
	if addr != checksummed && addr != strings.ToLower(checksummed) {
		// Allow both checksummed and all-lowercase; reject mixed case that doesn't match EIP-55
		if addr != strings.ToLower(addr) {
			return "", 0, fmt.Errorf("ethereum address checksum mismatch in did:pkh: got %q, expected %q", addr, checksummed)
		}
	}

	return checksummed, chainID, nil
}
