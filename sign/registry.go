package sign

import (
	"fmt"
	"strings"
	"sync"
)

var (
	// verifiers holds registered Verifier implementations keyed by algorithm name (lowercase).
	verifiers   = make(map[string]Verifier)
	verifiersMu sync.RWMutex
)

// RegisterVerifier registers a Verifier implementation for its algorithm.
// The algorithm name from v.Alg() is stored in lowercase for case-insensitive lookup.
// Registering the same algorithm twice replaces the previous implementation.
//
// Built-in algorithms (ed25519, eip191) are registered automatically at package init.
// This function is primarily for extensions or testing custom verifiers.
func RegisterVerifier(v Verifier) {
	verifiersMu.Lock()
	defer verifiersMu.Unlock()
	verifiers[strings.ToLower(v.Alg())] = v
}

// GetVerifier returns the Verifier implementation for the given algorithm name.
// The lookup is case-insensitive. Returns ErrUnsupportedAlg if no verifier is registered.
func GetVerifier(alg string) (Verifier, error) {
	verifiersMu.RLock()
	defer verifiersMu.RUnlock()
	v, ok := verifiers[strings.ToLower(alg)]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnsupportedAlg, alg)
	}
	return v, nil
}

// SupportedAlgorithms returns a sorted list of all registered algorithm names.
// Useful for documentation, error messages, or validation.
func SupportedAlgorithms() []string {
	verifiersMu.RLock()
	defer verifiersMu.RUnlock()
	algs := make([]string, 0, len(verifiers))
	for alg := range verifiers {
		algs = append(algs, alg)
	}
	return algs
}
