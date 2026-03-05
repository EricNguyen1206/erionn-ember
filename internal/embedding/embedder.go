package embedding

import "context"

// Embedder generates float32 vectors from text.
type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
	Dim() int
}

// ZeroEmbedder returns zero-vectors. Used as placeholder until ONNX is configured.
// Exact hash-match caching works fully. Semantic similarity search will not.
type ZeroEmbedder struct{ dim int }

func NewZeroEmbedder(dim int) Embedder { return &ZeroEmbedder{dim: dim} }

func (z *ZeroEmbedder) Embed(_ context.Context, _ string) ([]float32, error) {
	return make([]float32, z.dim), nil
}
func (z *ZeroEmbedder) Dim() int { return z.dim }
