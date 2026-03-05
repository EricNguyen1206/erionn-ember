package cache

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"
)

// Config holds SemanticCache configuration.
type Config struct {
	MaxElements         int
	SimilarityThreshold float32 // [0,1] — converted to max Hamming bits internally
	DefaultTTL          time.Duration
}

// maxHammingBits converts similarity threshold to max allowed differing bits.
// threshold=0.85 → 64*(1-0.85) ≈ 9 bits.
func (c Config) maxHammingBits() int {
	return int(float32(64) * (1.0 - c.SimilarityThreshold))
}

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

// SemanticCache orchestrates:
//
//	Fast path  → xxhash exact lookup       (~0µs)
//	Slow path  → SimHash Hamming scan      (~N µs, O(n) over stored entries)
//
// Zero dependencies: no model files, no CGO, no external services.
type SemanticCache struct {
	cfg        Config
	normalizer *Normalizer
	compressor *Compressor
	store      *MetadataStore
	simhasher  *SimHasher
	hits       atomic.Int64
	misses     atomic.Int64
	total      atomic.Int64
	nextID     atomic.Int64
}

// New creates a SemanticCache.
func New(cfg Config) *SemanticCache {
	return &SemanticCache{
		cfg:        cfg,
		normalizer: NewNormalizer(),
		compressor: NewCompressor(),
		store:      NewMetadataStore(cfg.MaxElements),
		simhasher:  NewSimHasher(),
	}
}

// Get looks up prompt. Returns (result, true) on hit, (nil, false) on miss.
func (c *SemanticCache) Get(_ context.Context, prompt string, threshold float32) (*GetResult, bool) {
	c.total.Add(1)
	if threshold == 0 {
		threshold = c.cfg.SimilarityThreshold
	}
	maxBits := int(float32(64) * (1.0 - threshold))

	normalized := c.normalizer.Normalize(prompt)
	hash := c.normalizer.Hash(normalized)

	// ── Fast path: exact xxhash match ────────────────────────────────────
	if e := c.store.FindByHash(hash); e != nil {
		resp, err := c.compressor.Decompress(e.CompressedResponse, e.OriginalResponseSize)
		if err == nil {
			c.hits.Add(1)
			return &GetResult{Response: resp, Similarity: 1.0, ExactMatch: true, CachedAt: e.CreatedAt}, true
		}
	}

	// ── Slow path: SimHash Hamming distance scan ──────────────────────────
	queryFP := c.simhasher.Hash(normalized)
	entries := c.store.ScanAll()

	var bestEntry *Entry
	bestBits := maxBits + 1 // "worse than threshold" sentinel

	for _, e := range entries {
		d := HammingDistance(queryFP, e.SimHash)
		if d < bestBits {
			bestBits = d
			bestEntry = e
		}
	}

	if bestEntry != nil && bestBits <= maxBits {
		if e := c.store.FindByHash(bestEntry.PromptHash); e != nil {
			resp, err := c.compressor.Decompress(e.CompressedResponse, e.OriginalResponseSize)
			if err == nil {
				c.hits.Add(1)
				sim := Similarity(queryFP, e.SimHash)
				return &GetResult{Response: resp, Similarity: sim, ExactMatch: false, CachedAt: e.CreatedAt}, true
			}
		}
	}

	c.misses.Add(1)
	return nil, false
}

// Set stores prompt + response in cache.
func (c *SemanticCache) Set(_ context.Context, prompt, response string, ttl time.Duration) (string, error) {
	if ttl == 0 {
		ttl = c.cfg.DefaultTTL
	}
	normalized := c.normalizer.Normalize(prompt)
	hash := c.normalizer.Hash(normalized)
	simhash := c.simhasher.Hash(normalized)

	compPrompt := c.compressor.Compress(prompt)
	compResp := c.compressor.Compress(response)

	id := fmt.Sprintf("%d", c.nextID.Add(1))
	now := time.Now()
	c.store.Set(hash, &Entry{
		ID:                   id,
		PromptHash:           hash,
		SimHash:              simhash,
		NormalizedPrompt:     normalized,
		CompressedPrompt:     compPrompt,
		CompressedResponse:   compResp,
		OriginalPromptSize:   len(prompt),
		OriginalResponseSize: len(response),
		CreatedAt:            now,
		LastAccessed:         now,
	}, ttl)
	return id, nil
}

// Delete removes entry by prompt.
func (c *SemanticCache) Delete(prompt string) bool {
	hash := c.normalizer.Hash(c.normalizer.Normalize(prompt))
	return c.store.Delete(hash)
}

// Stats returns current statistics.
func (c *SemanticCache) Stats() Stats {
	total := c.total.Load()
	hits := c.hits.Load()
	var hitRate float64
	if total > 0 {
		hitRate = float64(hits) / float64(total)
	}
	entries, _ := c.store.Stats()
	return Stats{
		TotalEntries: entries,
		CacheHits:    hits,
		CacheMisses:  c.misses.Load(),
		TotalQueries: total,
		HitRate:      hitRate,
	}
}
