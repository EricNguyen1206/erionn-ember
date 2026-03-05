package index

// SearchResult is a result from a vector search.
type SearchResult struct {
	ID       int     // insertion index (0-based)
	Distance float32 // lower = more similar (1 - cosine for unit vectors)
}

// VectorIndex is the interface for nearest-neighbor search.
type VectorIndex interface {
	Add(vec []float32) (int, error)
	Search(query []float32, k int) ([]SearchResult, error)
	Len() int
}
