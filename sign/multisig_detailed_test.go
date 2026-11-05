package sign

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/display-protocol/dp1-go/playlist"
)

// mixedGoldenDoc builds a document whose "signatures" array holds, in order: a valid Ed25519
// entry, an Ed25519 entry with the signature bytes zeroed, a valid EIP-191 entry, an entry with
// an unregistered alg, an entry whose sig is not base64, and an entry asserting a different
// payload_hash. One of every failure mode the device-side classifier distinguishes, interleaved
// with entries that pass so a per-entry report has to be right about both.
func mixedGoldenDoc(t *testing.T) (raw []byte, entries []playlist.Signature) {
	t.Helper()

	badSig := goldenEd25519Entry()
	badSig.Sig = base64.RawURLEncoding.EncodeToString(make([]byte, ed25519.SignatureSize))

	badAlg := goldenEd25519Entry()
	badAlg.Alg = "secp256r1"

	badB64 := goldenEd25519Entry()
	badB64.Sig = "not base64!!"

	badHash := goldenEd25519Entry()
	badHash.PayloadHash = "sha256:" + strings.Repeat("00", 32)

	entries = []playlist.Signature{
		goldenEd25519Entry(),
		badSig,
		goldenEthEntry(),
		badAlg,
		badB64,
		badHash,
	}
	return signedGoldenDoc(t, entries...), entries
}

// TestDetailed_MatchesPerEntryVerify is the equivalence test the caching change rests on: for
// every entry of a mixed document, the error the single-pass call reports must be the very error
// VerifyMultiSignature returns for that same entry and document. If caching the digest changed
// any outcome, it shows up here as a differing error identity.
func TestDetailed_MatchesPerEntryVerify(t *testing.T) {
	t.Parallel()
	raw, entries := mixedGoldenDoc(t)

	results, err := VerifyMultiSignaturesJSONDetailed(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != len(entries) {
		t.Fatalf("got %d results, want %d", len(results), len(entries))
	}

	for i, r := range results {
		if r.Index != i {
			t.Errorf("results[%d].Index = %d", i, r.Index)
		}
		if r.Signature != entries[i] {
			t.Errorf("results[%d].Signature = %+v, want %+v", i, r.Signature, entries[i])
		}
		want := VerifyMultiSignature(raw, entries[i])
		switch {
		case want == nil && r.Err != nil:
			t.Errorf("results[%d].Err = %v, want nil", i, r.Err)
		case want != nil && r.Err == nil:
			t.Errorf("results[%d].Err = nil, want %v", i, want)
		case want != nil && r.Err != nil && r.Err.Error() != want.Error():
			t.Errorf("results[%d].Err = %q, want %q", i, r.Err, want)
		}
		if r.OK() != (r.Err == nil) {
			t.Errorf("results[%d].OK() = %v but Err = %v", i, r.OK(), r.Err)
		}
	}
}

// TestDetailed_ErrorsAreClassifiable checks the mixed document entry by entry against the
// sentinels a consumer branches on. This is what replaces feral-controld's re-verification loop:
// one call, and every failure already carries the error that labels it.
func TestDetailed_ErrorsAreClassifiable(t *testing.T) {
	t.Parallel()
	raw, _ := mixedGoldenDoc(t)

	results, err := VerifyMultiSignaturesJSONDetailed(raw)
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		index int
		name  string
		check func(error) bool
	}{
		{0, "valid ed25519", func(e error) bool { return e == nil }},
		{1, "zeroed ed25519 sig", func(e error) bool { return errors.Is(e, ErrSigInvalid) }},
		{2, "valid eip191", func(e error) bool { return e == nil }},
		{3, "unregistered alg", func(e error) bool { return errors.Is(e, ErrUnsupportedAlg) }},
		{4, "non-base64 sig", func(e error) bool { return errors.Is(e, ErrSigInvalid) }},
		{5, "payload_hash mismatch", func(e error) bool {
			return errors.Is(e, ErrPayloadHashMismatch) && strings.Contains(e.Error(), "payload_hash")
		}},
	}
	for _, c := range cases {
		if !c.check(results[c.index].Err) {
			t.Errorf("entry %d (%s): err = %v", c.index, c.name, results[c.index].Err)
		}
	}
}

// TestDetailed_AgreesWithOldAPI keeps the two functions from drifting: the same document must
// yield the same verdict and the same failed entries, in the same order, through both.
func TestDetailed_AgreesWithOldAPI(t *testing.T) {
	t.Parallel()

	docs := map[string][]byte{}
	mixed, _ := mixedGoldenDoc(t)
	docs["mixed"] = mixed
	docs["all_valid"] = signedGoldenDoc(t, goldenEd25519Entry(), goldenEthEntry())

	onlyBad := goldenEd25519Entry()
	onlyBad.Sig = base64.RawURLEncoding.EncodeToString(make([]byte, ed25519.SignatureSize))
	docs["all_failed"] = signedGoldenDoc(t, onlyBad)

	for name, raw := range docs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ok, failed, err := VerifyMultiSignaturesJSON(raw)
			if err != nil {
				t.Fatal(err)
			}
			results, derr := VerifyMultiSignaturesJSONDetailed(raw)
			if derr != nil {
				t.Fatal(derr)
			}

			var wantFailed []playlist.Signature
			allOK := true
			for _, r := range results {
				if !r.OK() {
					allOK = false
					wantFailed = append(wantFailed, r.Signature)
				}
			}
			if ok != allOK {
				t.Fatalf("ok = %v, detailed says %v", ok, allOK)
			}
			if len(failed) != len(wantFailed) {
				t.Fatalf("failed has %d entries, detailed has %d", len(failed), len(wantFailed))
			}
			for i := range failed {
				if failed[i] != wantFailed[i] {
					t.Fatalf("failed[%d] = %+v, detailed says %+v", i, failed[i], wantFailed[i])
				}
			}
			// ok is an AND over entries, and failed is nil (not empty) when ok.
			if ok && failed != nil {
				t.Fatalf("ok but failed = %+v", failed)
			}
		})
	}
}

// TestDetailed_DocumentLevelErrors pins the detailed call's error returns to the old call's: the
// same inputs produce an error rather than results, including ErrNoSignatures for a missing or
// empty array.
func TestDetailed_DocumentLevelErrors(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		raw     string
		wantErr error // nil means "some error, identity unspecified"
	}{
		"missing_signatures":   {`{"dpVersion":"1.1.0","title":"x","items":[]}`, ErrNoSignatures},
		"empty_array":          {`{"signatures":[]}`, ErrNoSignatures},
		"null_signatures":      {`{"signatures":null}`, ErrNoSignatures},
		"invalid_json":         {`{`, nil},
		"signatures_not_array": {`{"signatures":{"alg":"ed25519"}}`, nil},
		"top_level_array":      {`[{"signatures":[]}]`, nil},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			results, err := VerifyMultiSignaturesJSONDetailed([]byte(c.raw))
			if err == nil {
				t.Fatalf("expected an error, got results = %+v", results)
			}
			if results != nil {
				t.Fatalf("results must be nil alongside an error, got %+v", results)
			}
			if c.wantErr != nil && !errors.Is(err, c.wantErr) {
				t.Fatalf("err = %v, want %v", err, c.wantErr)
			}
			// The old call must reject the same input the same way.
			ok, failed, oldErr := VerifyMultiSignaturesJSON([]byte(c.raw))
			if ok || failed != nil || oldErr == nil {
				t.Fatalf("VerifyMultiSignaturesJSON: ok=%v failed=%v err=%v", ok, failed, oldErr)
			}
			if err.Error() != oldErr.Error() {
				t.Fatalf("detailed err %q != old err %q", err, oldErr)
			}
		})
	}
}

// recordingVerifier captures every digest it is handed, so a test can assert that each entry of
// one document was checked against the same 32 bytes — the property that makes computing the
// digest once correct rather than merely faster.
type recordingVerifier struct {
	alg     string
	digests [][32]byte
}

func (v *recordingVerifier) Alg() string { return v.alg }

func (v *recordingVerifier) VerifySignature(_ string, _ []byte, digest [32]byte) error {
	v.digests = append(v.digests, digest)
	return nil
}

func TestDetailed_EveryEntrySeesTheDocumentDigest(t *testing.T) {
	// Not parallel: it registers an algorithm in the package-level verifier registry.
	rec := &recordingVerifier{alg: "testrecord"}
	RegisterVerifier(rec)

	entry := goldenEd25519Entry()
	entry.Alg = rec.alg
	raw := signedGoldenDoc(t, entry, entry, entry, entry)

	// payload_hash is checked before the verifier is reached, so it must be the document's own.
	want, err := signingDigest(raw)
	if err != nil {
		t.Fatal(err)
	}

	results, err := VerifyMultiSignaturesJSONDetailed(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 4 {
		t.Fatalf("got %d results, want 4", len(results))
	}
	if len(rec.digests) != 4 {
		t.Fatalf("verifier saw %d digests, want 4", len(rec.digests))
	}
	for i, got := range rec.digests {
		if got != want {
			t.Errorf("entry %d was verified against %x, want the document digest %x", i, got, want)
		}
	}
}

// TestDetailed_SignaturesKeyIsStrippedNotRead guards the premise of the whole change: the digest
// is independent of the "signatures" array, so adding entries cannot change what any entry is
// checked against.
func TestDetailed_SignaturesKeyIsStrippedNotRead(t *testing.T) {
	t.Parallel()
	one := signedGoldenDoc(t, goldenEd25519Entry())
	many := signedGoldenDoc(t, goldenEd25519Entry(), goldenEthEntry(), goldenEd25519Entry())

	hOne, err := PayloadHashString(one)
	if err != nil {
		t.Fatal(err)
	}
	hMany, err := PayloadHashString(many)
	if err != nil {
		t.Fatal(err)
	}
	if hOne != hMany || hOne != goldenPayloadHash {
		t.Fatalf("payload hashes diverged: one=%s many=%s unsigned=%s", hOne, hMany, goldenPayloadHash)
	}

	// A legacy top-level "signature" field is stripped too.
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(many, &doc); err != nil {
		t.Fatal(err)
	}
	doc["signature"] = json.RawMessage(`"` + goldenLegacySig + `"`)
	withLegacy, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	hLegacy, err := PayloadHashString(withLegacy)
	if err != nil {
		t.Fatal(err)
	}
	if hLegacy != goldenPayloadHash {
		t.Fatalf("payload hash changed when a legacy signature field was present: %s", hLegacy)
	}
}
