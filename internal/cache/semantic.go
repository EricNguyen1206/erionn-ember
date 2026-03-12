package cache

import (
	"context"
	"errors"
	"fmt"
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

// SemanticCache orchestrates prompt-based caching with semantic similarity matching.
//
//	Fast path  -> xxhash exact lookup       (~0us)
//	Slow path  -> BM25+Jaccard token scan   (~N us, O(n) over stored entries)
//
// Zero dependencies: no model files, no CGO, no external services.
type SemanticCache struct {
	cfg        Config
	normalizer *Normalizer
	compressor *Compressor
	store      *MetadataStore
	scorer     *Scorer
	hits       atomic.Int64
	misses     atomic.Int64
	total      atomic.Int64
	nextID     atomic.Int64
}

// New creates a new SemanticCache instance.
func New(cfg Config) *SemanticCache {
	return &SemanticCache{
		cfg:        cfg,
		normalizer: NewNormalizer(),
		compressor: NewCompressor(),
		store:      NewMetadataStore(cfg.MaxElements),
		scorer:     NewScorer(),
	}
}

// Get looks up a prompt in the cache. Returns (result, true) on hit, (nil, false) on miss.
func (c *SemanticCache) Get(_ context.Context, prompt string, threshold float32) (*GetResult, bool) {
	c.total.Add(1)
	if threshold == 0 {
		threshold = c.cfg.SimilarityThreshold
	}

	normalized := c.normalizer.Normalize(prompt)
	hash := c.normalizer.Hash(normalized)

	// Fast path: exact xxhash match.
	if entry, expired := c.store.FindByHashWithExpired(hash); entry != nil {
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
		c.removeEntryTokens(expired)
	}

	queryTokens := Tokenize(normalized)
	if len(queryTokens) == 0 {
		c.misses.Add(1)
		return nil, false
	}

	// Slow path: BM25 + Jaccard similarity scan.
	allEntries, expiredEntries := c.store.ScanAllLive()
	c.removeEntriesTokens(expiredEntries)

	var bestEntry *Entry
	var bestScore float32

	for _, entry := range allEntries {
		score := c.scorer.Score(queryTokens, entry.Tokens)
		if score > bestScore {
			bestScore = score
			bestEntry = entry
		}
	}

	if bestEntry != nil && bestScore >= threshold {
		// Re-fetch to refresh LRU metadata and ensure the entry still exists.
		if entry, expired := c.store.FindByHashWithExpired(bestEntry.PromptHash); entry != nil {
			response, err := c.compressor.Decompress(entry.CompressedResponse, entry.OriginalResponseSize)
			if err == nil {
				c.hits.Add(1)
				return &GetResult{
					Response:   response,
					Similarity: bestScore,
					ExactMatch: false,
					CachedAt:   entry.CreatedAt,
				}, true
			}
		} else {
			c.removeEntryTokens(expired)
		}
	}

	c.misses.Add(1)
	return nil, false
}

// Set stores a prompt + response in the cache.
func (c *SemanticCache) Set(_ context.Context, prompt, response string, ttl time.Duration) (string, error) {
	if prompt == "" {
		return "", ErrEmptyPrompt
	}
	if response == "" {
		return "", ErrEmptyResponse
	}
	if ttl == 0 {
		ttl = c.cfg.DefaultTTL
	}

	normalized := c.normalizer.Normalize(prompt)
	hash := c.normalizer.Hash(normalized)
	tokens := Tokenize(normalized)

	compPrompt := c.compressor.Compress(prompt)
	compResp := c.compressor.Compress(response)

	id := fmt.Sprintf("%d", c.nextID.Add(1))
	now := time.Now()
	entry := &Entry{
		ID:                   id,
		PromptHash:           hash,
		Tokens:               tokens,
		NormalizedPrompt:     normalized,
		CompressedPrompt:     compPrompt,
		CompressedResponse:   compResp,
		OriginalPromptSize:   len(prompt),
		OriginalResponseSize: len(response),
		CreatedAt:            now,
		LastAccessed:         now,
	}

	replaced, evicted := c.store.Set(hash, entry, ttl)
	c.removeEntryTokens(replaced)
	c.removeEntryTokens(evicted)
	c.scorer.UpdateIDF(tokens)

	return id, nil
}

// Delete removes an entry from the cache by its original prompt.
func (c *SemanticCache) Delete(prompt string) bool {
	normalized := c.normalizer.Normalize(prompt)
	hash := c.normalizer.Hash(normalized)
	entry, deleted := c.store.DeleteEntry(hash)
	if deleted {
		c.removeEntryTokens(entry)
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
	c.removeEntriesTokens(expired)

	return Stats{
		TotalEntries: entries,
		CacheHits:    hits,
		CacheMisses:  c.misses.Load(),
		TotalQueries: total,
		HitRate:      hitRate,
	}
}

func (c *SemanticCache) removeEntriesTokens(entries []*Entry) {
	for _, entry := range entries {
		c.removeEntryTokens(entry)
	}
}

func (c *SemanticCache) removeEntryTokens(entry *Entry) {
	if entry == nil || len(entry.Tokens) == 0 {
		return
	}
	c.scorer.RemoveDoc(entry.Tokens)
}
