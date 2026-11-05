package sign

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/display-protocol/dp1-go/playlist"
)

// benchItemCount gives a document of roughly 1 MiB — small enough that `go test -bench` stays
// quick, large enough that one JCS canonicalization dominates the Ed25519 verifications it is
// being compared against. Feral File's real series playlists run 4.9–8.9 MiB, so on those the
// gap these benchmarks show is several times wider.
const benchItemCount = 6000

// benchSignedDoc returns a ~1 MiB playlist carrying entries Ed25519 signature entries. When
// valid is false every entry's signature bytes are zeroed, which exercises the failure path:
// before this change that path was the worse one, because a caller wanting to know which entry
// failed had to verify each entry a second time.
func benchSignedDoc(b *testing.B, entries int, valid bool) []byte {
	b.Helper()

	items := make([]playlist.PlaylistItem, benchItemCount)
	duration := 300.0
	for i := range items {
		items[i] = playlist.PlaylistItem{
			ID:       fmt.Sprintf("3%07d-a45f-4c1c-8080-e93a192adccc", i),
			Slug:     fmt.Sprintf("bench-item-%07d", i),
			Title:    fmt.Sprintf("Benchmark artwork number %07d, with a title long enough to be realistic", i),
			Source:   fmt.Sprintf("https://example.com/artworks/%07d/index.html?seed=%d", i, i),
			Duration: &duration,
			License:  "open",
		}
	}
	p := playlist.Playlist{
		DPVersion: "1.1.0",
		ID:        "385f79b6-a45f-4c1c-8080-e93a192adccc",
		Slug:      "bench-playlist",
		Title:     "Bench",
		Created:   "2025-01-01T00:00:00Z",
		Items:     items,
	}

	body, err := json.Marshal(p)
	if err != nil {
		b.Fatal(err)
	}

	sigs := make([]playlist.Signature, 0, entries)
	for i := 0; i < entries; i++ {
		// A distinct key per entry: a real multi-signed document is signed by distinct
		// curators/feeds, and identical entries would let an accidental per-entry cache hide
		// what these benchmarks measure.
		_, priv, err := ed25519.GenerateKey(nil)
		if err != nil {
			b.Fatal(err)
		}
		sig, err := SignMultiEd25519(body, priv, playlist.RoleCurator, "2025-01-01T00:00:00Z")
		if err != nil {
			b.Fatal(err)
		}
		if !valid {
			sig.Sig = base64.RawURLEncoding.EncodeToString(make([]byte, ed25519.SignatureSize))
		}
		sigs = append(sigs, sig)
	}
	p.Signatures = sigs

	raw, err := json.Marshal(p)
	if err != nil {
		b.Fatal(err)
	}
	return raw
}

var benchEntryCounts = []int{1, 2, 4, 16}

// BenchmarkVerifyMultiSignaturesJSON is the cost claim this branch makes, in runnable form:
// ns/op should stay roughly flat as entries grows, because the document is canonicalized once and
// each further entry adds only an Ed25519 verification (tens of microseconds against the
// milliseconds a canonicalization of this size costs). Before this change ns/op grew linearly in
// entries, at two canonicalizations each.
//
//	go test ./sign/ -run '^$' -bench 'VerifyMultiSignatures' -benchtime 10x
func BenchmarkVerifyMultiSignaturesJSON(b *testing.B) {
	for _, entries := range benchEntryCounts {
		raw := benchSignedDoc(b, entries, true)
		b.Run(fmt.Sprintf("entries=%d", entries), func(b *testing.B) {
			b.SetBytes(int64(len(raw)))
			b.ReportAllocs()
			for b.Loop() {
				ok, _, err := VerifyMultiSignaturesJSON(raw)
				if err != nil || !ok {
					b.Fatalf("ok=%v err=%v", ok, err)
				}
			}
		})
	}
}

// BenchmarkVerifyMultiSignaturesJSONDetailed should track BenchmarkVerifyMultiSignaturesJSON
// closely: reporting every entry's error costs a slice, not another pass over the document.
func BenchmarkVerifyMultiSignaturesJSONDetailed(b *testing.B) {
	for _, entries := range benchEntryCounts {
		raw := benchSignedDoc(b, entries, true)
		b.Run(fmt.Sprintf("entries=%d", entries), func(b *testing.B) {
			b.SetBytes(int64(len(raw)))
			b.ReportAllocs()
			for b.Loop() {
				results, err := VerifyMultiSignaturesJSONDetailed(raw)
				if err != nil {
					b.Fatal(err)
				}
				for _, r := range results {
					if !r.OK() {
						b.Fatalf("entry %d: %v", r.Index, r.Err)
					}
				}
			}
		})
	}
}

// BenchmarkVerifyFailedThenReverify measures what a caller used to pay to label a failure: verify
// the document, then call VerifyMultiSignature again for each failed entry to recover its error.
// This is the pattern VerifyMultiSignaturesJSONDetailed replaces, so the gap between this and
// BenchmarkVerifyMultiSignaturesJSONDetailed/entries=N on an all-failing document is the saving
// a consumer gets by switching.
func BenchmarkVerifyFailedThenReverify(b *testing.B) {
	for _, entries := range benchEntryCounts {
		raw := benchSignedDoc(b, entries, false)
		b.Run(fmt.Sprintf("entries=%d", entries), func(b *testing.B) {
			b.SetBytes(int64(len(raw)))
			b.ReportAllocs()
			for b.Loop() {
				_, failed, err := VerifyMultiSignaturesJSON(raw)
				if err != nil {
					b.Fatal(err)
				}
				for _, sig := range failed {
					if verr := VerifyMultiSignature(raw, sig); verr == nil {
						b.Fatal("expected a per-entry error")
					}
				}
			}
		})
	}
}

// BenchmarkVerifyMultiSignaturesJSONFailing is the same failing document through one pass, for
// comparison with BenchmarkVerifyFailedThenReverify.
func BenchmarkVerifyMultiSignaturesJSONFailing(b *testing.B) {
	for _, entries := range benchEntryCounts {
		raw := benchSignedDoc(b, entries, false)
		b.Run(fmt.Sprintf("entries=%d", entries), func(b *testing.B) {
			b.SetBytes(int64(len(raw)))
			b.ReportAllocs()
			for b.Loop() {
				results, err := VerifyMultiSignaturesJSONDetailed(raw)
				if err != nil {
					b.Fatal(err)
				}
				if len(results) != entries {
					b.Fatalf("got %d results, want %d", len(results), entries)
				}
			}
		})
	}
}

// BenchmarkVerifyMultiSignature is the single-entry baseline: one call canonicalizes once, so
// this is the floor every benchmark above is measured against.
func BenchmarkVerifyMultiSignature(b *testing.B) {
	raw := benchSignedDoc(b, 1, true)
	var envelope struct {
		Signatures []playlist.Signature `json:"signatures"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		b.Fatal(err)
	}
	sig := envelope.Signatures[0]
	b.SetBytes(int64(len(raw)))
	b.ReportAllocs()
	for b.Loop() {
		if err := VerifyMultiSignature(raw, sig); err != nil {
			b.Fatal(err)
		}
	}
}
