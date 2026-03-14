package cache

import "context"

// Embedder produces embeddings for normalized prompt text.
type Embedder interface {
	Embed(context.Context, string) ([]float32, error)
	Dimension() int
}
