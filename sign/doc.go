/*
Package sign implements DP-1 signing payload construction (JCS, UTF-8, LF-terminated octets) and Ed25519 verification.

Pipeline (§7.1): strip top-level signature fields, RFC 8785 JCS, one LF — then SHA-256. Ed25519 (legacy and
v1.1 multi-sig) signs that 32-byte digest. Use PayloadHashString / VerifyPayloadHash for the assertion field.
payload_hash ("sha256:…") asserts the same digest but is not fed to Ed25519 verify.
Legacy v1.0.x uses the same digest pipeline, with the signature embedded as ed25519:<hex>.

Additional algorithms (eip191, ecdsa-secp256k1, ecdsa-p256) are not implemented yet; VerifyMultiEd25519 returns
ErrUnsupportedAlg for non-ed25519 alg values until those are added.
*/
package sign
