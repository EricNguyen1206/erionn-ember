package cache

import "errors"

var (
	ErrNilVectorEntry          = errors.New("vector index: nil entry")
	ErrEmptyVector             = errors.New("vector index: empty vector")
	ErrVectorDimensionMismatch = errors.New("vector index: vector dimension mismatch")
)

// VectorSearchResult identifies a nearest-neighbor search hit.
type VectorSearchResult struct {
	EntryID      string
	NamespaceKey string
	PromptHash   uint64
	Score        float32
}

// VectorIndexStats reports aggregate vector index state.
type VectorIndexStats struct {
	Namespaces int
	Vectors    int
	Dimension  int
}

// VectorIndex abstracts vector storage and nearest-neighbor search.
type VectorIndex interface {
	Insert(entry *Entry) error
	Delete(namespaceKey string, promptHash uint64) bool
	Search(namespaceKey string, query []float32, limit int) []VectorSearchResult
	Stats() VectorIndexStats
}
