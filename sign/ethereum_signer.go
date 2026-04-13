package sign

import (
	"crypto/ecdsa"
	"fmt"

	"github.com/ethereum/go-ethereum/crypto"

	"github.com/display-protocol/dp1-go/playlist"
)

// EthereumSigner implements signature creation for the eip191 algorithm.
// It signs the DP-1 digest using Ethereum's personal_sign format (EIP-191 version 0x45)
// and produces did:pkh identifiers with the appropriate chain ID.
type EthereumSigner struct {
	privateKey *ecdsa.PrivateKey
	chainID    int
}

// NewEthereumSigner creates an EthereumSigner from an ECDSA private key and chain ID.
//
// The chainID identifies the EVM-compatible chain:
//   - 1: Ethereum mainnet
//   - 5: Goerli testnet (deprecated)
//   - 11155111: Sepolia testnet
//   - 137: Polygon
//   - 42161: Arbitrum One
//   - 8453: Base
//   - See https://chainlist.org for complete list
//
// The chainID is included in the did:pkh identifier but is NOT cryptographically bound
// to the signature (known limitation of personal_sign). Applications requiring explicit
// chain-binding should validate the chainId or consider EIP-712 structured data.
func NewEthereumSigner(priv *ecdsa.PrivateKey, chainID int) *EthereumSigner {
	return &EthereumSigner{
		privateKey: priv,
		chainID:    chainID,
	}
}

// Alg returns "eip191" to match playlist.AlgEIP191.
func (s *EthereumSigner) Alg() string {
	return playlist.AlgEIP191
}

// Sign creates an Ethereum personal_sign (EIP-191) signature over the DP-1 signing digest.
//
// The signing process:
//  1. Sign the 32-byte digest using crypto.Sign (applies personal_sign prefix internally)
//  2. Derive the Ethereum address from the private key's public key
//  3. Create did:pkh identifier: did:pkh:eip155:{chainID}:{address}
//
// Returns:
//   - kid: did:pkh identifier with EIP-55 checksummed address
//   - sigBytes: 65-byte ECDSA signature (r[32] + s[32] + v[1] where v is 0 or 1)
//   - err: Non-nil if signing or DID creation fails
//
// Note: The returned signature v value will be 0 or 1. Some Ethereum implementations
// expect 27/28; adjust if needed for your use case.
func (s *EthereumSigner) Sign(digest [32]byte) (string, []byte, error) {
	// Sign the digest (crypto.Sign applies personal_sign prefix)
	sigBytes, err := crypto.Sign(digest[:], s.privateKey)
	if err != nil {
		return "", nil, fmt.Errorf("ethereum sign: %w", err)
	}

	// Derive address from public key
	pub, ok := s.privateKey.Public().(*ecdsa.PublicKey)
	if !ok {
		return "", nil, fmt.Errorf("private key has unexpected public key type %T", s.privateKey.Public())
	}
	addr := crypto.PubkeyToAddress(*pub)

	// Create did:pkh identifier
	kid, err := EthereumAddressToDIDPKH(addr.Hex(), s.chainID)
	if err != nil {
		return "", nil, err
	}

	return kid, sigBytes, nil
}
