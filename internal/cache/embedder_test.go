package cache_test

import (
	"context"
	"testing"

	"github.com/EricNguyen1206/erion-ember/internal/cache"
)

type stubEmbedder struct {
	vectors map[string][]float32
	dim     int
}

func (s stubEmbedder) Embed(_ context.Context, text string) ([]float32, error) {
	return s.vectors[text], nil
}

func (s stubEmbedder) Dimension() int {
	return s.dim
}

var _ cache.Embedder = stubEmbedder{}

func TestEmbedderContract(t *testing.T) {
	var embedder cache.Embedder = stubEmbedder{
		vectors: map[string][]float32{
			"hello": {0.1, 0.2, 0.3},
		},
		dim: 3,
	}

	if got := embedder.Dimension(); got != 3 {
		t.Fatalf("Dimension() = %d, want 3", got)
	}

	vector, err := embedder.Embed(context.Background(), "hello")
	if err != nil {
		t.Fatalf("Embed() error = %v", err)
	}

	if len(vector) != 3 {
		t.Fatalf("len(Embed()) = %d, want 3", len(vector))
	}

	if vector[0] != 0.1 || vector[1] != 0.2 || vector[2] != 0.3 {
		t.Fatalf("Embed() = %v, want [0.1 0.2 0.3]", vector)
	}
}
