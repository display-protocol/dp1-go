package sign

import (
	"crypto/ed25519"
	"encoding/json"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"

	"github.com/display-protocol/dp1-go/playlist"
)

func TestEthereumSignerVerifierRoundTrip(t *testing.T) {
	t.Parallel()

	// Generate key
	priv, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}

	chainID := 1
	p := playlist.Playlist{
		DPVersion: "1.1.0",
		Title:     "Ethereum Test",
		Items:     []playlist.PlaylistItem{{Source: "https://example.com/art.html"}},
	}
	raw, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}

	// Sign
	sig, err := SignMultiEIP191(raw, priv, chainID, playlist.RoleCurator, "2026-04-13T10:00:00Z")
	if err != nil {
		t.Fatal(err)
	}

	// Validate signature fields
	if sig.Alg != playlist.AlgEIP191 {
		t.Errorf("sig.Alg = %q, want %q", sig.Alg, playlist.AlgEIP191)
	}

	expectedAddr := crypto.PubkeyToAddress(priv.PublicKey).Hex()
	expectedKid, err := EthereumAddressToDIDPKH(expectedAddr, chainID)
	if err != nil {
		t.Fatal(err)
	}
	if sig.Kid != expectedKid {
		t.Errorf("sig.Kid = %q, want %q", sig.Kid, expectedKid)
	}

	if sig.PayloadHash == "" {
		t.Error("sig.PayloadHash is empty")
	}

	if sig.Sig == "" {
		t.Error("sig.Sig is empty")
	}

	// Verify
	if err := VerifyMultiSignature(raw, sig); err != nil {
		t.Fatalf("VerifyMultiSignature() error = %v", err)
	}
}

func TestEthereumVerifierInvalidSignature(t *testing.T) {
	t.Parallel()

	priv, _ := crypto.GenerateKey()
	p := playlist.Playlist{
		DPVersion: "1.1.0",
		Title:     "Test",
		Items:     []playlist.PlaylistItem{{Source: "https://example.com"}},
	}
	raw, _ := json.Marshal(p)

	sig, err := SignMultiEIP191(raw, priv, 1, playlist.RoleFeed, "2026-04-13T10:00:00Z")
	if err != nil {
		t.Fatal(err)
	}

	// Corrupt the signature (flip a byte)
	sigBytes := []byte(sig.Sig)
	if len(sigBytes) > 0 {
		sigBytes[0] ^= 0xFF
		sig.Sig = string(sigBytes)
	}

	err = VerifyMultiSignature(raw, sig)
	if err == nil {
		t.Fatal("expected verification to fail with corrupted signature")
	}
}

func TestEthereumVerifierWrongDocument(t *testing.T) {
	t.Parallel()

	priv, _ := crypto.GenerateKey()
	p1 := playlist.Playlist{
		DPVersion: "1.1.0",
		Title:     "Original",
		Items:     []playlist.PlaylistItem{{Source: "https://example.com/1"}},
	}
	raw1, _ := json.Marshal(p1)

	sig, err := SignMultiEIP191(raw1, priv, 1, playlist.RoleCurator, "2026-04-13T10:00:00Z")
	if err != nil {
		t.Fatal(err)
	}

	// Try to verify with different document
	p2 := playlist.Playlist{
		DPVersion: "1.1.0",
		Title:     "Different",
		Items:     []playlist.PlaylistItem{{Source: "https://example.com/2"}},
	}
	raw2, _ := json.Marshal(p2)

	err = VerifyMultiSignature(raw2, sig)
	if err == nil {
		t.Fatal("expected verification to fail with different document")
	}
}

func TestEthereumVerifierMultipleChains(t *testing.T) {
	t.Parallel()

	priv, _ := crypto.GenerateKey()
	p := playlist.Playlist{
		DPVersion: "1.1.0",
		Title:     "Multi-chain",
		Items:     []playlist.PlaylistItem{{Source: "https://example.com"}},
	}
	raw, _ := json.Marshal(p)

	chains := []struct {
		name    string
		chainID int
	}{
		{"mainnet", 1},
		{"polygon", 137},
		{"arbitrum", 42161},
		{"base", 8453},
	}

	for _, chain := range chains {
		t.Run(chain.name, func(t *testing.T) {
			sig, err := SignMultiEIP191(raw, priv, chain.chainID, playlist.RoleFeed, "2026-04-13T10:00:00Z")
			if err != nil {
				t.Fatalf("SignMultiEIP191() error = %v", err)
			}

			// Verify the kid contains the correct chain ID
			addr, chainID, err := EthereumAddressFromDIDPKH(sig.Kid)
			if err != nil {
				t.Fatalf("EthereumAddressFromDIDPKH() error = %v", err)
			}
			if chainID != chain.chainID {
				t.Errorf("chainID = %d, want %d", chainID, chain.chainID)
			}

			expectedAddr := crypto.PubkeyToAddress(priv.PublicKey).Hex()
			if addr != expectedAddr {
				t.Errorf("address = %q, want %q", addr, expectedAddr)
			}

			// Verify signature
			if err := VerifyMultiSignature(raw, sig); err != nil {
				t.Fatalf("VerifyMultiSignature() error = %v", err)
			}
		})
	}
}

func TestEthereumVerifierInvalidKid(t *testing.T) {
	t.Parallel()

	priv, _ := crypto.GenerateKey()
	p := playlist.Playlist{
		DPVersion: "1.1.0",
		Title:     "Test",
		Items:     []playlist.PlaylistItem{{Source: "https://example.com"}},
	}
	raw, _ := json.Marshal(p)

	sig, err := SignMultiEIP191(raw, priv, 1, playlist.RoleCurator, "2026-04-13T10:00:00Z")
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		kid  string
	}{
		{"wrong_did_method", "did:key:z6MkhaXgBZDvotDkL5257faiztiGiC2QtKLGpbnnEGta2doK"},
		{"wrong_namespace", "did:pkh:bip122:000000000019d6689c085ae165831e93:128Lkh3S7CkDTBZ8W7BbpsN3YYizJMp8p6"},
		{"invalid_address", "did:pkh:eip155:1:0xinvalid"},
		{"missing_parts", "did:pkh:eip155:1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sig.Kid = tt.kid
			err := VerifyMultiSignature(raw, sig)
			if err == nil {
				t.Fatal("expected verification to fail with invalid kid")
			}
		})
	}
}

func TestMixedEd25519AndEthereumSignatures(t *testing.T) {
	t.Parallel()

	// Generate keys
	_, ed25519Priv, _ := ed25519.GenerateKey(nil)
	ethPriv, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}

	p := playlist.Playlist{
		DPVersion: "1.1.0",
		Title:     "Mixed Signatures",
		Items:     []playlist.PlaylistItem{{Source: "https://example.com"}},
	}
	raw, _ := json.Marshal(p)

	// Sign with Ed25519
	sig1, err := SignMultiEd25519(raw, ed25519Priv, playlist.RoleCurator, "2026-04-13T10:00:00Z")
	if err != nil {
		t.Fatal(err)
	}

	// Sign with Ethereum
	sig2, err := SignMultiEIP191(raw, ethPriv, 1, playlist.RoleFeed, "2026-04-13T10:00:00Z")
	if err != nil {
		t.Fatal(err)
	}

	// Add both signatures to document
	p.Signatures = []playlist.Signature{sig1, sig2}
	raw, _ = json.Marshal(p)

	// Verify all signatures
	ok, failed, err := VerifyPlaylistSignatures(raw)
	if err != nil {
		t.Fatalf("VerifyPlaylistSignatures() error = %v", err)
	}
	if !ok {
		t.Fatalf("verification failed: %d signatures failed", len(failed))
	}
	if len(failed) != 0 {
		t.Errorf("expected 0 failed signatures, got %d", len(failed))
	}
}

func TestEthereumVerifierCrossChainReplay(t *testing.T) {
	t.Parallel()

	priv, _ := crypto.GenerateKey()
	p := playlist.Playlist{
		DPVersion: "1.1.0",
		Title:     "Cross-chain Test",
		Items:     []playlist.PlaylistItem{{Source: "https://example.com"}},
	}
	raw, _ := json.Marshal(p)

	// Sign for mainnet
	sigMainnet, err := SignMultiEIP191(raw, priv, 1, playlist.RoleCurator, "2026-04-13T10:00:00Z")
	if err != nil {
		t.Fatal(err)
	}

	// Create a signature for polygon with the same sig bytes (simulating replay)
	sigPolygon := sigMainnet
	addr := crypto.PubkeyToAddress(priv.PublicKey).Hex()
	polygonKid, _ := EthereumAddressToDIDPKH(addr, 137)
	sigPolygon.Kid = polygonKid

	// Both should verify because the signature is valid for the same address
	// This demonstrates the known cross-chain replay limitation
	if err := VerifyMultiSignature(raw, sigMainnet); err != nil {
		t.Errorf("mainnet signature verification failed: %v", err)
	}
	if err := VerifyMultiSignature(raw, sigPolygon); err != nil {
		t.Errorf("polygon signature verification failed: %v", err)
	}

	t.Log("Cross-chain replay is possible (known limitation): same signature verifies for different chainIDs")
}
