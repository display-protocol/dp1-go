/*
Package sign implements DP-1 signing payload construction (JCS, UTF-8, LF-terminated octets) and signature verification.

# Signing Pipeline (DP-1 §7.1)

All algorithms sign the same canonical digest:

 1. Strip top-level "signature" and "signatures" fields from JSON
 2. Apply RFC 8785 JCS (JSON Canonicalization Scheme)
 3. Append single LF (0x0A) to the canonical UTF-8 bytes
 4. Compute SHA-256 hash → 32-byte digest

The payload_hash field ("sha256:...") asserts this digest but is not fed to the signature algorithm.

# Supported Algorithms

Ed25519 (legacy v1.0.x and v1.1+ multi-sig):
  - Algorithm: "ed25519"
  - Key ID format: did:key (W3C, multicodec ed25519-pub 0xed01, base58btc)
  - Signature: 64 bytes, signs the 32-byte digest directly
  - Functions: [SignMultiEd25519], [VerifyMultiEd25519], [VerifyMultiSignature]

Ethereum EIP-191 personal_sign (v1.1+ multi-sig):
  - Algorithm: "eip191"
  - Key ID format: did:pkh:eip155:{chainID}:{address} (CAIP-10, EIP-55 checksum)
  - Signature: 65 bytes (ECDSA secp256k1: r[32] + s[32] + v[1]), signs the 32-byte digest
  - Functions: [SignMultiEIP191], [VerifyMultiSignature]
  - Chain support: All EVM-compatible chains (Ethereum, Polygon, Arbitrum, Base, etc.)

# Replay Attack Protections

Cross-document replay: PREVENTED by payload_hash validation. A signature cannot be moved
between documents with different content.

Cross-chain replay (Ethereum): NOT PREVENTED for personal_sign (EIP-191). The same signature
is valid across all EVM chains for a given address. This is acceptable for curator attestations
where the identity (address) and content (payload_hash) are the meaningful bindings.
Applications requiring explicit chain-binding should validate the chainId from the kid field
or use EIP-712 structured data (future extension).

Temporal replay: NOT PREVENTED. The ts field indicates signature time but freshness is not
enforced by the SDK. Applications should validate timestamp requirements based on their use case.

# Multi-Signature Documents (v1.1+)

Use [VerifyMultiSignaturesJSON] to verify all signatures in a document's "signatures" array.
The function returns ok=true when all signatures verify, or ok=false with a list of failed
signatures. Convenience wrappers: [VerifyPlaylistSignatures], [VerifyPlaylistGroupSignatures],
[VerifyChannelSignatures].

Documents can mix Ed25519 and Ethereum signatures. Each signature is verified independently
using its algorithm-specific verifier from the registry.

# Algorithm Registry

Built-in verifiers (ed25519, eip191) are registered automatically at package init.
Extensions can add custom algorithms via [RegisterVerifier] and [Verifier] interface.

# Legacy v1.0.x Single Signature

The legacy single-signature format (top-level "signature" field with "ed25519:<hex>" value)
uses the same digest pipeline. See [VerifyPlaylistLegacy] for v1.0.x verification.

# References

  - DP-1 §7.1: Signing and Verification
  - RFC 8785: JSON Canonicalization Scheme (JCS)
  - W3C did:key: https://w3c-ccg.github.io/did-method-key/
  - W3C did:pkh: https://github.com/w3c-ccg/did-pkh
  - CAIP-10: https://chainagnostic.org/CAIPs/caip-10
  - EIP-191: https://eips.ethereum.org/EIPS/eip-191
  - EIP-55: https://eips.ethereum.org/EIPS/eip-55
*/
package sign
