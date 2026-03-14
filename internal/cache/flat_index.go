package cache

import (
	"fmt"
	"math"
	"sort"
	"sync"
)

type flatVectorEntry struct {
	entryID      string
	namespaceKey string
	promptHash   uint64
	vector       []float32
	norm         float64
}

// FlatIndex stores vectors in memory and searches them with a linear scan.
type FlatIndex struct {
	mu                  sync.RWMutex
	namespaceDimensions map[string]int
	namespaces          map[string]map[uint64]flatVectorEntry
}

// NewFlatIndex creates an empty flat vector index.
func NewFlatIndex() *FlatIndex {
	return &FlatIndex{
		namespaceDimensions: make(map[string]int),
		namespaces:          make(map[string]map[uint64]flatVectorEntry),
	}
}

// Insert upserts an entry into the namespace partition keyed by prompt hash.
func (i *FlatIndex) Insert(entry *Entry) error {
	if entry == nil {
		return ErrNilVectorEntry
	}
	if len(entry.Vector) == 0 {
		return ErrEmptyVector
	}

	stored := flatVectorEntry{
		entryID:      entry.ID,
		namespaceKey: entry.NamespaceKey,
		promptHash:   entry.PromptHash,
		vector:       append([]float32(nil), entry.Vector...),
		norm:         vectorNorm(entry.Vector),
	}

	i.mu.Lock()
	defer i.mu.Unlock()

	dimension := i.namespaceDimensions[entry.NamespaceKey]
	if dimension == 0 {
		i.namespaceDimensions[entry.NamespaceKey] = len(entry.Vector)
	} else if len(entry.Vector) != dimension {
		return fmt.Errorf("%w: namespace %q got %d want %d", ErrVectorDimensionMismatch, entry.NamespaceKey, len(entry.Vector), dimension)
	}

	partition := i.namespaces[entry.NamespaceKey]
	if partition == nil {
		partition = make(map[uint64]flatVectorEntry)
		i.namespaces[entry.NamespaceKey] = partition
	}

	partition[entry.PromptHash] = stored
	return nil
}

// Delete removes a vector from a namespace partition.
func (i *FlatIndex) Delete(namespaceKey string, promptHash uint64) bool {
	i.mu.Lock()
	defer i.mu.Unlock()

	partition := i.namespaces[namespaceKey]
	if partition == nil {
		return false
	}
	if _, ok := partition[promptHash]; !ok {
		return false
	}

	delete(partition, promptHash)
	if len(partition) == 0 {
		delete(i.namespaces, namespaceKey)
		delete(i.namespaceDimensions, namespaceKey)
	}

	return true
}

// Search scans a namespace partition and returns the top cosine matches.
func (i *FlatIndex) Search(namespaceKey string, query []float32, limit int) []VectorSearchResult {
	if limit <= 0 || len(query) == 0 {
		return nil
	}

	queryNorm := vectorNorm(query)
	if queryNorm == 0 {
		return nil
	}

	i.mu.RLock()
	partition := i.namespaces[namespaceKey]
	if len(partition) == 0 {
		i.mu.RUnlock()
		return nil
	}

	results := make([]VectorSearchResult, 0, len(partition))
	for _, candidate := range partition {
		if len(candidate.vector) != len(query) || candidate.norm == 0 {
			continue
		}

		score := cosineSimilarity(candidate.vector, candidate.norm, query, queryNorm)
		results = append(results, VectorSearchResult{
			EntryID:      candidate.entryID,
			NamespaceKey: candidate.namespaceKey,
			PromptHash:   candidate.promptHash,
			Score:        score,
		})
	}
	i.mu.RUnlock()

	if len(results) == 0 {
		return nil
	}

	sort.Slice(results, func(a, b int) bool {
		if results[a].Score == results[b].Score {
			if results[a].PromptHash == results[b].PromptHash {
				return results[a].EntryID < results[b].EntryID
			}
			return results[a].PromptHash < results[b].PromptHash
		}
		return results[a].Score > results[b].Score
	})

	if limit < len(results) {
		results = results[:limit]
	}

	return results
}

// Stats returns aggregate flat index metadata.
func (i *FlatIndex) Stats() VectorIndexStats {
	i.mu.RLock()
	defer i.mu.RUnlock()

	stats := VectorIndexStats{
		Namespaces: len(i.namespaces),
		Dimension:  i.sharedDimensionLocked(),
	}
	for _, partition := range i.namespaces {
		stats.Vectors += len(partition)
	}

	return stats
}

func (i *FlatIndex) sharedDimensionLocked() int {
	shared := 0
	for namespaceKey := range i.namespaces {
		dimension := i.namespaceDimensions[namespaceKey]
		if dimension == 0 {
			continue
		}
		if shared == 0 {
			shared = dimension
			continue
		}
		if shared != dimension {
			return 0
		}
	}

	return shared
}

func vectorNorm(vector []float32) float64 {
	var sum float64
	for _, value := range vector {
		sum += float64(value) * float64(value)
	}
	return math.Sqrt(sum)
}

func cosineSimilarity(left []float32, leftNorm float64, right []float32, rightNorm float64) float32 {
	if leftNorm == 0 || rightNorm == 0 || len(left) != len(right) {
		return 0
	}

	var dot float64
	for idx := range left {
		dot += float64(left[idx]) * float64(right[idx])
	}

	return float32(dot / (leftNorm * rightNorm))
}
