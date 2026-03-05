package cache

import (
	"strings"
	"unicode"

	"github.com/cespare/xxhash/v2"
)

// Normalizer normalizes prompt text and computes fast hashes.
type Normalizer struct{}

func NewNormalizer() *Normalizer { return &Normalizer{} }

// Normalize lowercases, trims, and collapses internal whitespace.
func (n *Normalizer) Normalize(text string) string {
	text = strings.ToLower(strings.TrimSpace(text))
	var b strings.Builder
	b.Grow(len(text))
	prevSpace := false
	for _, r := range text {
		if unicode.IsSpace(r) {
			if !prevSpace {
				b.WriteRune(' ')
			}
			prevSpace = true
		} else {
			b.WriteRune(r)
			prevSpace = false
		}
	}
	return b.String()
}

// Hash returns a 64-bit xxhash of the text.
func (n *Normalizer) Hash(text string) uint64 {
	return xxhash.Sum64String(text)
}
