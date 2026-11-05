package sign

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"

	"github.com/display-protocol/dp1-go/playlist"
)

// Wire-compatibility fixtures, captured by running v0.7.2 (the last release before verification
// was reduced to one canonicalization per document) over goldenDocJSON with the fixed keys below.
//
// They exist because that change moved where the §7.1 digest is computed. The digest is provably
// entry-independent, so caching it cannot alter what is signed — but a coding error in the caching
// would, silently and in a way only cross-version checks catch. These constants are that check:
// as long as they hold, a document signed by v0.7.2 verifies here and a document signed here
// verifies under v0.7.2. Both algorithms are deterministic (Ed25519 per RFC 8032, secp256k1 via
// RFC 6979 in go-ethereum), so the signature bytes are reproducible, not merely verifiable.
//
// Never regenerate these from the current code to make a test pass: that would erase the only
// evidence the wire format has not moved. A deliberate §7.1 change is a compatibility break and
// belongs in CHANGELOG.md with new fixtures captured from the new rule.
const (
	goldenDocJSON = `{"dpVersion":"1.1.0","id":"385f79b6-a45f-4c1c-8080-e93a192adccc","slug":"golden-playlist","title":"Golden","created":"2025-01-01T00:00:00Z","items":[{"id":"285f79b6-a45f-4c1c-8080-e93a192adccc","source":"https://example.com/a.html","duration":300,"license":"open"}]}`

	goldenPayloadHash = "sha256:ecb017915638709686d04f3d794ea6b74635c810df41122acfc842ee25390e9a"

	goldenEd25519Kid = "did:key:z6MkneMkZqwqRiU5mJzSG3kDwzt9P8C59N4NGTfBLfSGE7c7"
	goldenEd25519Sig = "Liu7QH0-WN_mEDHQxJUvYiTNxc7edJVcnCe2kYf7oLISAFWyJi63T9qTVJvrnwOEMHpP1Dhaa5QFPJYFPiF0Bw"

	goldenLegacySig = "ed25519:2e2bbb407d3e58dfe61031d0c4952f6224cdc5cede74955c9c27b69187fba0b2120055b2262eb74fda93549beb9f0384307a4fd4385a6b94053c96053e217407"

	// Private key from go-ethereum's own test vectors; chain ID 1.
	goldenEthPrivHex = "4c0883a69102937d6231471b5dbb6204fe512961708279f3b3b8f1f7e0c26e1c"
	goldenEthKid     = "did:pkh:eip155:1:0x2726000d91a402adbE609f9696E6d92256626AaC"
	goldenEthSig     = "GuJJhnDKV81YCMxQlyQNrI0YqCYfZCjCnq1YACePIxBW6j8PDEsDNCQBuBMO4g-dW6wsuKWtkJxtFfLFhcvffgE"

	goldenTs = "2025-01-01T00:00:00Z"
)

// goldenEd25519Key is the Ed25519 key behind goldenEd25519Kid: seed bytes 0x01..0x20.
func goldenEd25519Key(t *testing.T) ed25519.PrivateKey {
	t.Helper()
	seed := make([]byte, ed25519.SeedSize)
	for i := range seed {
		seed[i] = byte(i + 1)
	}
	return ed25519.NewKeyFromSeed(seed)
}

func goldenEthKey(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	priv, err := crypto.HexToECDSA(goldenEthPrivHex)
	if err != nil {
		t.Fatal(err)
	}
	return priv
}

// goldenEd25519Entry / goldenEthEntry are the signature entries as v0.7.2 produced them.
func goldenEd25519Entry() playlist.Signature {
	return playlist.Signature{
		Alg:         playlist.AlgEd25519,
		Kid:         goldenEd25519Kid,
		Ts:          goldenTs,
		PayloadHash: goldenPayloadHash,
		Role:        playlist.RoleCurator,
		Sig:         goldenEd25519Sig,
	}
}

func goldenEthEntry() playlist.Signature {
	return playlist.Signature{
		Alg:         playlist.AlgEIP191,
		Kid:         goldenEthKid,
		Ts:          goldenTs,
		PayloadHash: goldenPayloadHash,
		Role:        playlist.RoleFeed,
		Sig:         goldenEthSig,
	}
}

// signedGoldenDoc returns goldenDocJSON with the given entries under "signatures". It goes through
// a map so the test exercises the real shape a signed document arrives in: re-marshaling reorders
// keys, which §7.1 verification must be indifferent to because JCS normalizes key order.
func signedGoldenDoc(t *testing.T, sigs ...playlist.Signature) []byte {
	t.Helper()
	var doc map[string]json.RawMessage
	if err := json.Unmarshal([]byte(goldenDocJSON), &doc); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(sigs)
	if err != nil {
		t.Fatal(err)
	}
	doc["signatures"] = encoded
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

// TestGolden_SigningMatchesV072 locks what this SDK produces: payload_hash and both algorithms'
// signature bytes must be byte-identical to v0.7.2's for the same input, so a document signed here
// verifies under v0.7.2 (and every other DP-1 implementation that agreed with it).
func TestGolden_SigningMatchesV072(t *testing.T) {
	t.Parallel()
	raw := []byte(goldenDocJSON)

	got, err := PayloadHashString(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got != goldenPayloadHash {
		t.Fatalf("PayloadHashString = %q, want v0.7.2 value %q", got, goldenPayloadHash)
	}

	edSig, err := SignMultiEd25519(raw, goldenEd25519Key(t), playlist.RoleCurator, goldenTs)
	if err != nil {
		t.Fatal(err)
	}
	if edSig != goldenEd25519Entry() {
		t.Fatalf("SignMultiEd25519 = %+v, want v0.7.2 entry %+v", edSig, goldenEd25519Entry())
	}

	ethSig, err := SignMultiEIP191(raw, goldenEthKey(t), 1, playlist.RoleFeed, goldenTs)
	if err != nil {
		t.Fatal(err)
	}
	if ethSig != goldenEthEntry() {
		t.Fatalf("SignMultiEIP191 = %+v, want v0.7.2 entry %+v", ethSig, goldenEthEntry())
	}

	legacy, err := SignLegacyEd25519(raw, goldenEd25519Key(t))
	if err != nil {
		t.Fatal(err)
	}
	if legacy != goldenLegacySig {
		t.Fatalf("SignLegacyEd25519 = %q, want v0.7.2 value %q", legacy, goldenLegacySig)
	}
}

// TestGolden_VerifiesV072Signatures is the other direction: signatures produced by v0.7.2, taken
// as opaque bytes, still verify here — through the single-entry call, the (ok, failed) call and
// the per-entry call alike, and for both algorithms.
func TestGolden_VerifiesV072Signatures(t *testing.T) {
	t.Parallel()
	raw := signedGoldenDoc(t, goldenEd25519Entry(), goldenEthEntry())

	for _, sig := range []playlist.Signature{goldenEd25519Entry(), goldenEthEntry()} {
		if err := VerifyMultiSignature(raw, sig); err != nil {
			t.Fatalf("VerifyMultiSignature(%s) = %v, want nil", sig.Alg, err)
		}
	}
	if err := VerifyMultiEd25519(raw, goldenEd25519Entry()); err != nil {
		t.Fatalf("VerifyMultiEd25519 = %v, want nil", err)
	}

	ok, failed, err := VerifyPlaylistSignatures(raw)
	if err != nil || !ok || failed != nil {
		t.Fatalf("VerifyPlaylistSignatures: ok=%v failed=%v err=%v", ok, failed, err)
	}

	results, err := VerifyMultiSignaturesJSONDetailed(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}
	for i, r := range results {
		if !r.OK() {
			t.Fatalf("results[%d] (%s) = %v, want nil", i, r.Signature.Alg, r.Err)
		}
	}

	pub := goldenEd25519Key(t).Public().(ed25519.PublicKey)
	if err := VerifyLegacyEd25519([]byte(goldenDocJSON), goldenLegacySig, pub); err != nil {
		t.Fatalf("VerifyLegacyEd25519 = %v, want nil", err)
	}
}

// TestGolden_ErrorIdentities pins the error each kind of bad entry produces. These identities are
// a cross-repo contract: feral-controld's sigverify classifies a verification failure by
// errors.Is on ErrUnsupportedAlg and ErrSigInvalid, and by the "payload_hash" substring for a
// mismatch, so none of them may drift.
func TestGolden_ErrorIdentities(t *testing.T) {
	t.Parallel()
	raw := signedGoldenDoc(t, goldenEd25519Entry())

	t.Run("payload_hash_mismatch", func(t *testing.T) {
		t.Parallel()
		sig := goldenEd25519Entry()
		sig.PayloadHash = "sha256:" + strings.Repeat("00", 32)
		err := VerifyMultiSignature(raw, sig)
		if !errors.Is(err, ErrPayloadHashMismatch) {
			t.Fatalf("err = %v, want ErrPayloadHashMismatch", err)
		}
		// The substring, not just the sentinel: the consumer predates the sentinel.
		if !strings.Contains(err.Error(), "payload_hash") {
			t.Fatalf("err text %q must contain \"payload_hash\"", err.Error())
		}
		if errors.Is(err, ErrSigInvalid) {
			t.Fatalf("a payload_hash mismatch must not be ErrSigInvalid; got %v", err)
		}
	})

	t.Run("tampered_document", func(t *testing.T) {
		t.Parallel()
		// Same entry, a document whose content moved: payload_hash no longer describes it.
		tampered := strings.Replace(goldenDocJSON, `"title":"Golden"`, `"title":"Tampered"`, 1)
		var doc map[string]json.RawMessage
		if err := json.Unmarshal([]byte(tampered), &doc); err != nil {
			t.Fatal(err)
		}
		encoded, err := json.Marshal([]playlist.Signature{goldenEd25519Entry()})
		if err != nil {
			t.Fatal(err)
		}
		doc["signatures"] = encoded
		rawTampered, err := json.Marshal(doc)
		if err != nil {
			t.Fatal(err)
		}
		if err := VerifyMultiSignature(rawTampered, goldenEd25519Entry()); !errors.Is(err, ErrPayloadHashMismatch) {
			t.Fatalf("err = %v, want ErrPayloadHashMismatch", err)
		}
	})

	t.Run("wrong_key", func(t *testing.T) {
		t.Parallel()
		// A valid did:key for a different Ed25519 key: the digest matches, the signature does not.
		otherPub, _, err := ed25519.GenerateKey(nil)
		if err != nil {
			t.Fatal(err)
		}
		otherKid, err := Ed25519DIDKey(otherPub)
		if err != nil {
			t.Fatal(err)
		}
		sig := goldenEd25519Entry()
		sig.Kid = otherKid
		if err := VerifyMultiSignature(raw, sig); !errors.Is(err, ErrSigInvalid) {
			t.Fatalf("err = %v, want ErrSigInvalid", err)
		}
	})

	t.Run("unsupported_alg", func(t *testing.T) {
		t.Parallel()
		sig := goldenEd25519Entry()
		sig.Alg = "secp256r1"
		if err := VerifyMultiSignature(raw, sig); !errors.Is(err, ErrUnsupportedAlg) {
			t.Fatalf("err = %v, want ErrUnsupportedAlg", err)
		}
	})

	t.Run("bad_base64_sig", func(t *testing.T) {
		t.Parallel()
		sig := goldenEd25519Entry()
		sig.Sig = "not base64!!"
		if err := VerifyMultiSignature(raw, sig); !errors.Is(err, ErrSigInvalid) {
			t.Fatalf("err = %v, want ErrSigInvalid", err)
		}
	})

	t.Run("short_sig_bytes", func(t *testing.T) {
		t.Parallel()
		sig := goldenEd25519Entry()
		sig.Sig = goldenEd25519Sig[:40]
		if err := VerifyMultiSignature(raw, sig); !errors.Is(err, ErrSigInvalid) {
			t.Fatalf("err = %v, want ErrSigInvalid", err)
		}
	})
}
