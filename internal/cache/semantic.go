package cache

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/EricNguyen1206/erion-ember/internal/embedding"
	"github.com/EricNguyen1206/erion-ember/internal/index"
)

// Config holds SemanticCache configuration.
type Config struct {
	Dim                 int
	MaxElements         int
	SimilarityThreshold float32
	DefaultTTL          time.Duration
	EmbedWorkers        int
}

func DefaultConfig() Config {
	return Config{
		Dim:                 384,
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

// SemanticCache orchestrates: exact hash lookup → embed → vector search.
type SemanticCache struct {
	cfg        Config
	normalizer *Normalizer
	compressor *Compressor
	store      *MetadataStore
	idx        index.VectorIndex
	pool       *embedding.Pool
	writeMu    sync.Mutex // serializes idx.Add + store.Set
	hits       atomic.Int64
	misses     atomic.Int64
	total      atomic.Int64
}

// New creates a SemanticCache.
func New(cfg Config, embedder embedding.Embedder, vidx index.VectorIndex) *SemanticCache {
	return &SemanticCache{
		cfg:        cfg,
		normalizer: NewNormalizer(),
		compressor: NewCompressor(),
		store:      NewMetadataStore(cfg.MaxElements),
		idx:        vidx,
		pool:       embedding.NewPool(cfg.EmbedWorkers, embedder),
	}
}

// Get looks up prompt. Returns (result, true) on hit, (nil, false) on miss.
func (c *SemanticCache) Get(ctx context.Context, prompt string, threshold float32) (*GetResult, bool) {
	c.total.Add(1)
	if threshold == 0 {
		threshold = c.cfg.SimilarityThreshold
	}

	normalized := c.normalizer.Normalize(prompt)
	hash := c.normalizer.Hash(normalized)

	// ── Fast path: exact hash match (~0.1ms, no ONNX) ──
	if e := c.store.FindByHash(hash); e != nil {
		resp, err := c.compressor.Decompress(e.CompressedResponse, e.OriginalResponseSize)
		if err == nil {
			c.hits.Add(1)
			return &GetResult{Response: resp, Similarity: 1.0, ExactMatch: true, CachedAt: e.CreatedAt}, true
		}
	}

	// ── Slow path: embed → vector search ──
	vec, err := c.pool.Embed(ctx, prompt)
	if err != nil {
		c.misses.Add(1)
		return nil, false
	}
	results, err := c.idx.Search(vec, 5)
	if err != nil {
		c.misses.Add(1)
		return nil, false
	}
	for _, r := range results {
		sim := 1.0 - r.Distance
		if sim >= threshold {
			if e := c.store.FindByVectorID(r.ID); e != nil {
				resp, err := c.compressor.Decompress(e.CompressedResponse, e.OriginalResponseSize)
				if err == nil {
					c.hits.Add(1)
					return &GetResult{Response: resp, Similarity: sim, ExactMatch: false, CachedAt: e.CreatedAt}, true
				}
			}
		}
	}
	c.misses.Add(1)
	return nil, false
}

// Set embeds prompt and stores (prompt, response) in cache.
func (c *SemanticCache) Set(ctx context.Context, prompt, response string, ttl time.Duration) (string, error) {
	if ttl == 0 {
		ttl = c.cfg.DefaultTTL
	}
	normalized := c.normalizer.Normalize(prompt)
	hash := c.normalizer.Hash(normalized)

	vec, err := c.pool.Embed(ctx, prompt)
	if err != nil {
		return "", fmt.Errorf("embed: %w", err)
	}

	compPrompt := c.compressor.Compress(prompt)
	compResp := c.compressor.Compress(response)

	c.writeMu.Lock()
	vectorID, err := c.idx.Add(vec)
	if err != nil {
		c.writeMu.Unlock()
		return "", fmt.Errorf("index add: %w", err)
	}
	now := time.Now()
	id := fmt.Sprintf("%d", vectorID)
	c.store.Set(hash, &Entry{
		ID:                   id,
		VectorID:             vectorID,
		PromptHash:           hash,
		NormalizedPrompt:     normalized,
		CompressedPrompt:     compPrompt,
		CompressedResponse:   compResp,
		OriginalPromptSize:   len(prompt),
		OriginalResponseSize: len(response),
		CreatedAt:            now,
		LastAccessed:         now,
	}, ttl)
	c.writeMu.Unlock()

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

// Close releases pool workers.
func (c *SemanticCache) Close() { c.pool.Close() }
