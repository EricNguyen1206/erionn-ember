package index

import (
	"math"
	"sort"
	"sync"
)

// FlatIndex is a brute-force cosine similarity index (O(n) per query).
// Sufficient for ≤100k entries. Replace with HNSW for larger scale.
type FlatIndex struct {
	mu      sync.RWMutex
	dim     int
	vectors [][]float32
}

func NewFlatIndex(dim int) *FlatIndex {
	return &FlatIndex{dim: dim, vectors: make([][]float32, 0, 1024)}
}

func (f *FlatIndex) Add(vec []float32) (int, error) {
	unit := normalize(vec)
	f.mu.Lock()
	id := len(f.vectors)
	f.vectors = append(f.vectors, unit)
	f.mu.Unlock()
	return id, nil
}

func (f *FlatIndex) Search(query []float32, k int) ([]SearchResult, error) {
	unitQ := normalize(query)
	f.mu.RLock()
	defer f.mu.RUnlock()

	results := make([]SearchResult, 0, len(f.vectors))
	for i, v := range f.vectors {
		dist := float32(1.0 - dot(unitQ, v))
		results = append(results, SearchResult{ID: i, Distance: dist})
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].Distance < results[j].Distance
	})
	if k > len(results) {
		k = len(results)
	}
	return results[:k], nil
}

func (f *FlatIndex) Len() int {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return len(f.vectors)
}

func dot(a, b []float32) float64 {
	var s float64
	for i := range a {
		s += float64(a[i]) * float64(b[i])
	}
	return s
}

func normalize(v []float32) []float32 {
	var sum float64
	for _, x := range v {
		sum += float64(x) * float64(x)
	}
	out := make([]float32, len(v))
	if sum == 0 {
		return out
	}
	norm := math.Sqrt(sum)
	for i, x := range v {
		out[i] = float32(float64(x) / norm)
	}
	return out
}
