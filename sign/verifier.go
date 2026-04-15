package sign

// Verifier is the interface for algorithm-specific signature verification.
// Each algorithm (ed25519, eip191, etc.) implements this interface to handle
// its specific verification logic while sharing the common DP-1 signing digest.
type Verifier interface {
	// Alg returns the algorithm identifier (e.g., "ed25519", "eip191").
	// Must match the value in playlist.Signature.Alg (case-insensitive comparison).
	Alg() string

	// VerifySignature verifies a signature against the DP-1 signing digest.
	//
	// Parameters:
	//   kid: Key identifier (did:key for ed25519, did:pkh for ethereum)
	//   sigBytes: Decoded signature bytes (algorithm-specific format)
	//   digest: 32-byte SHA-256 hash of JCS(payload)+LF (DP-1 §7.1)
	//
	// Returns:
	//   nil if signature is valid
	//   ErrSigInvalid if signature verification fails
	//   other error if kid parsing or other validation fails
	VerifySignature(kid string, sigBytes []byte, digest [32]byte) error
}

// Signer is the interface for algorithm-specific signature creation.
// Each algorithm implements this to produce signatures over the DP-1 signing digest.
type Signer interface {
	// Alg returns the algorithm identifier (e.g., "ed25519", "eip191").
	Alg() string

	// Sign creates a signature over the DP-1 signing digest.
	//
	// Parameters:
	//   digest: 32-byte SHA-256 hash of JCS(payload)+LF (DP-1 §7.1)
	//
	// Returns:
	//   kid: Key identifier in appropriate DID format for this algorithm
	//   sigBytes: Raw signature bytes (algorithm-specific format)
	//   err: Non-nil on signing failure
	Sign(digest [32]byte) (kid string, sigBytes []byte, err error)
}
