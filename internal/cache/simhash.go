package cache

import (
	"math/bits"
	"strings"
	"unicode"

	"github.com/cespare/xxhash/v2"
)

// SimHasher computes 64-bit locality-sensitive fingerprints for text.
//
// Algorithm (Charikar SimHash):
//  1. Tokenize text into words
//  2. For each token: hash it → for each of 64 bit positions,
//     add +1 to v[i] if bit=1, else -1.
//  3. Final fingerprint: bit[i] = 1 if v[i] > 0, else 0.
//
// Key property: similar texts (sharing most tokens) produce fingerprints
// with small Hamming distance. Unlike neural embeddings:
//   - No model file, no CGO, no external service
//   - Deterministic and reproducible
//   - ~1µs per hash (vs ~2ms ONNX, ~15ms Ollama HTTP)
//   - Works best for near-duplicate detection (shared vocabulary)
type SimHasher struct{}

func NewSimHasher() *SimHasher { return &SimHasher{} }

// Hash returns a 64-bit SimHash fingerprint for the (already-normalised) text.
func (s *SimHasher) Hash(normalized string) uint64 {
	tokens := tokenize(normalized)
	if len(tokens) == 0 {
		return 0
	}

	var v [64]int32
	for _, tok := range tokens {
		h := xxhash.Sum64String(tok)
		for i := 0; i < 64; i++ {
			if (h>>uint(i))&1 == 1 {
				v[i]++
			} else {
				v[i]--
			}
		}
	}

	var fp uint64
	for i := 0; i < 64; i++ {
		if v[i] > 0 {
			fp |= 1 << uint(i)
		}
	}
	return fp
}

// Similarity returns a [0,1] similarity score based on Hamming distance.
// 1.0 = identical fingerprint, 0.0 = all 64 bits differ.
func Similarity(a, b uint64) float32 {
	diffBits := bits.OnesCount64(a ^ b)
	return float32(64-diffBits) / 64.0
}

// HammingDistance returns the number of differing bits (0–64).
func HammingDistance(a, b uint64) int {
	return bits.OnesCount64(a ^ b)
}

// tokenize splits text into lowercase words (already normalised → just split).
func tokenize(text string) []string {
	return strings.FieldsFunc(text, func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsPunct(r)
	})
}
