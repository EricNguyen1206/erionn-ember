package cache_test

import (
	"testing"

	"github.com/EricNguyen1206/erion-ember/internal/cache"
)

func TestNormalize(t *testing.T) {
	n := cache.NewNormalizer()
	cases := []struct{ in, want string }{
		{"Hello World", "hello world"},
		{"  spaces  ", "spaces"},
		{"UPPER CASE", "upper case"},
		{"multi   space", "multi space"},
		{"", ""},
	}
	for _, c := range cases {
		got := n.Normalize(c.in)
		if got != c.want {
			t.Errorf("Normalize(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestHashConsistency(t *testing.T) {
	n := cache.NewNormalizer()
	if n.Hash("hello") != n.Hash("hello") {
		t.Error("hash must be deterministic")
	}
}

func TestHashDifferent(t *testing.T) {
	n := cache.NewNormalizer()
	if n.Hash("foo") == n.Hash("bar") {
		t.Error("different inputs should differ")
	}
}
