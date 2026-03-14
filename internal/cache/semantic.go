package cache

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"
)

var (
	ErrEmptyPrompt   = errors.New("prompt is required")
	ErrEmptyResponse = errors.New("response is required")
)

// Config holds SemanticCache configuration.
type Config struct {
	MaxElements         int
	SimilarityThreshold float32 // [0,1]
	DefaultTTL          time.Duration
}

// DefaultConfig returns the default configuration for SemanticCache.
func DefaultConfig() Config {
	return Config{
		MaxElements:         100000,
		SimilarityThreshold: 0.85,
		DefaultTTL:          time.Hour,
	}
}

// GetResult is returned by Get on a cache hit.
type GetResult struct {
	Response   string
	Similarity float32
	ExactMatch bool
	CachedAt   time.Time
}

// Stats holds cache statistics.
type Stats struct {
	TotalEntries int
	CacheHits    int64
	CacheMisses  int64
	TotalQueries int64
	HitRate      float64
}

// SemanticCache orchestrates prompt-based caching with exact lookup first and
// optional namespace-scoped vector search as a semantic fallback.
type SemanticCache struct {
	cfg        Config
	normalizer *Normalizer
	compressor *Compressor
	store      *MetadataStore
	embedder   Embedder
	vectorIdx  VectorIndex
	hits       atomic.Int64
	misses     atomic.Int64
	total      atomic.Int64
	nextID     atomic.Int64
}

// New creates a new SemanticCache instance.
func New(cfg Config) *SemanticCache {
	return NewWithDependencies(cfg, nil, NewFlatIndex())
}

// NewWithDependencies creates a SemanticCache with optional semantic-search dependencies.
func NewWithDependencies(cfg Config, embedder Embedder, vectorIdx VectorIndex) *SemanticCache {
	if vectorIdx == nil {
		vectorIdx = NewFlatIndex()
	}

	return &SemanticCache{
		cfg:        cfg,
		normalizer: NewNormalizer(),
		compressor: NewCompressor(),
		store:      NewMetadataStore(cfg.MaxElements),
		embedder:   embedder,
		vectorIdx:  vectorIdx,
	}
}

// Get looks up a prompt in the cache. Returns (result, true) on hit, (nil, false) on miss.
func (c *SemanticCache) Get(ctx context.Context, prompt string, threshold float32) (*GetResult, bool) {
	return c.GetInNamespace(ctx, Namespace{}, prompt, threshold)
}

// GetInNamespace looks up a prompt within a namespace.
func (c *SemanticCache) GetInNamespace(ctx context.Context, namespace Namespace, prompt string, threshold float32) (*GetResult, bool) {
	c.total.Add(1)
	if threshold == 0 {
		threshold = c.cfg.SimilarityThreshold
	}

	namespaceKey := cacheNamespaceKey(namespace)
	normalized := c.normalizer.Normalize(prompt)
	hash := c.normalizer.Hash(normalized)

	if entry, expired := c.store.FindExactByHashWithExpired(namespaceKey, hash); entry != nil {
		response, err := c.compressor.Decompress(entry.CompressedResponse, entry.OriginalResponseSize)
		if err == nil {
			c.hits.Add(1)
			return &GetResult{
				Response:   response,
				Similarity: 1.0,
				ExactMatch: true,
				CachedAt:   entry.CreatedAt,
			}, true
		}
	} else {
		c.removeEntryIndexes(expired)
	}

	if c.embedder == nil || c.vectorIdx == nil || normalized == "" {
		c.misses.Add(1)
		return nil, false
	}

	queryVector, err := c.embedder.Embed(ctx, normalized)
	if err != nil || len(queryVector) == 0 {
		c.misses.Add(1)
		return nil, false
	}

	results := c.vectorIdx.Search(namespaceKey, queryVector, c.store.Len())
	for _, result := range results {
		if result.Score < threshold {
			break
		}

		if entry, expired := c.store.FindExactByHashWithExpired(result.NamespaceKey, result.PromptHash); entry != nil {
			response, err := c.compressor.Decompress(entry.CompressedResponse, entry.OriginalResponseSize)
			if err == nil {
				c.hits.Add(1)
				return &GetResult{
					Response:   response,
					Similarity: result.Score,
					ExactMatch: false,
					CachedAt:   entry.CreatedAt,
				}, true
			}
		} else {
			c.removeEntryIndexes(expired)
		}
	}

	c.misses.Add(1)
	return nil, false
}

// Set stores a prompt + response in the cache.
func (c *SemanticCache) Set(ctx context.Context, prompt, response string, ttl time.Duration) (string, error) {
	return c.SetInNamespace(ctx, Namespace{}, prompt, response, ttl)
}

// SetInNamespace stores a prompt + response pair inside a namespace.
func (c *SemanticCache) SetInNamespace(ctx context.Context, namespace Namespace, prompt, response string, ttl time.Duration) (string, error) {
	if prompt == "" {
		return "", ErrEmptyPrompt
	}
	if response == "" {
		return "", ErrEmptyResponse
	}
	if ttl == 0 {
		ttl = c.cfg.DefaultTTL
	}

	namespaceKey := cacheNamespaceKey(namespace)
	normalized := c.normalizer.Normalize(prompt)
	hash := c.normalizer.Hash(normalized)

	compPrompt := c.compressor.Compress(prompt)
	compResp := c.compressor.Compress(response)

	id := fmt.Sprintf("%d", c.nextID.Add(1))
	now := time.Now()
	entry := &Entry{
		ID:                   id,
		NamespaceKey:         namespaceKey,
		PromptHash:           hash,
		NormalizedPrompt:     normalized,
		CompressedPrompt:     compPrompt,
		CompressedResponse:   compResp,
		OriginalPromptSize:   len(prompt),
		OriginalResponseSize: len(response),
		CreatedAt:            now,
		LastAccessed:         now,
	}

	if c.embedder != nil {
		vector, err := c.embedder.Embed(ctx, normalized)
		if err != nil {
			slog.Debug("semantic cache embed failed; storing exact-only entry", "namespace", namespaceKey, "prompt_hash", hash, "error", err)
		} else if len(vector) > 0 {
			entry.Vector = vector
		}
	}

	replaced, evicted := c.store.SetExact(namespaceKey, hash, entry, ttl)
	c.removeEntryIndexes(replaced)
	c.removeEntryIndexes(evicted)

	if len(entry.Vector) > 0 && c.vectorIdx != nil {
		if err := c.vectorIdx.Insert(entry); err != nil {
			slog.Debug("semantic cache vector insert failed; storing exact-only entry", "namespace", namespaceKey, "prompt_hash", hash, "error", err)
		}
	}

	return id, nil
}

// Delete removes an entry from the cache by its original prompt.
func (c *SemanticCache) Delete(prompt string) bool {
	return c.DeleteInNamespace(Namespace{}, prompt)
}

// DeleteInNamespace removes an entry from a namespace by its original prompt.
func (c *SemanticCache) DeleteInNamespace(namespace Namespace, prompt string) bool {
	namespaceKey := cacheNamespaceKey(namespace)
	normalized := c.normalizer.Normalize(prompt)
	hash := c.normalizer.Hash(normalized)
	entry, deleted := c.store.DeleteExactEntry(namespaceKey, hash)
	if deleted {
		c.removeEntryIndexes(entry)
	}
	return deleted
}

// Stats returns current cache statistics.
func (c *SemanticCache) Stats() Stats {
	total := c.total.Load()
	hits := c.hits.Load()

	var hitRate float64
	if total > 0 {
		hitRate = float64(hits) / float64(total)
	}

	entries, _, expired := c.store.StatsLive()
	c.removeEntriesIndexes(expired)

	return Stats{
		TotalEntries: entries,
		CacheHits:    hits,
		CacheMisses:  c.misses.Load(),
		TotalQueries: total,
		HitRate:      hitRate,
	}
}

func (c *SemanticCache) removeEntriesIndexes(entries []*Entry) {
	for _, entry := range entries {
		c.removeEntryIndexes(entry)
	}
}

func (c *SemanticCache) removeEntryIndexes(entry *Entry) {
	if entry == nil || c.vectorIdx == nil {
		return
	}
	c.vectorIdx.Delete(entry.NamespaceKey, entry.PromptHash)
}

func cacheNamespaceKey(namespace Namespace) string {
	if namespace == (Namespace{}) {
		return ""
	}
	return NamespaceKey(namespace)
}
