package cache_test

import (
	"context"
	"testing"
	"time"

	"github.com/EricNguyen1206/erion-ember/internal/cache"
)

func newTestCache() *cache.SemanticCache {
	return cache.New(cache.Config{
		MaxElements:         100,
		SimilarityThreshold: 0.85,
		DefaultTTL:          time.Hour,
	})
}

// ── Exact match (fast path, xxhash) ──────────────────────────────────────

func TestSemanticCache_ExactHit(t *testing.T) {
	sc := newTestCache()
	ctx := context.Background()

	sc.Set(ctx, "What is Go?", "A language.", 0)
	result, ok := sc.Get(ctx, "What is Go?", 0)
	if !ok {
		t.Fatal("expected hit")
	}
	if !result.ExactMatch {
		t.Error("want exact_match=true")
	}
	if result.Response != "A language." {
		t.Errorf("wrong response: %q", result.Response)
	}
	if result.Similarity != 1.0 {
		t.Errorf("exact match similarity should be 1.0, got %f", result.Similarity)
	}
}

func TestSemanticCache_ExactMiss(t *testing.T) {
	sc := newTestCache()
	_, ok := sc.Get(context.Background(), "never stored", 0)
	if ok {
		t.Error("expected miss")
	}
}

func TestSemanticCache_NormalizeBeforeHash(t *testing.T) {
	sc := newTestCache()
	ctx := context.Background()
	sc.Set(ctx, "what is go?", "Go is a language.", 0)
	// Uppercase + extra spaces → same after normalisation
	result, ok := sc.Get(ctx, "  What IS Go?  ", 0)
	if !ok {
		t.Fatal("expected hit after normalisation")
	}
	if !result.ExactMatch {
		t.Error("normalised prompt should produce exact match")
	}
}

// ── SimHash semantic similarity (slow path) ───────────────────────────────

func TestSemanticCache_SimHashSimilarityHit(t *testing.T) {
	// SimHash is order-independent: same token set → (near-)identical fingerprint.
	// "go language fast compiled" and "compiled fast language go" share all tokens
	// → Hamming distance = 0 → similarity = 1.0 → HIT despite different word order.
	sc := newTestCache()
	ctx := context.Background()

	sc.Set(ctx, "go language fast compiled", "Cached response.", 0)

	// Different word order → different xxhash (exact miss) but same SimHash (semantic hit)
	result, ok := sc.Get(ctx, "compiled fast language go", 0.85)
	if !ok {
		t.Fatal("expected SimHash similarity hit")
	}
	if result.ExactMatch {
		t.Error("different word order should not be an exact match")
	}
	if result.Response != "Cached response." {
		t.Errorf("wrong response: %q", result.Response)
	}
	if result.Similarity < 0.85 {
		t.Errorf("similarity %f should be ≥ 0.85", result.Similarity)
	}
}

func TestSemanticCache_SimHashMiss_Unrelated(t *testing.T) {
	sc := newTestCache()
	ctx := context.Background()
	sc.Set(ctx, "weather forecast tomorrow rain", "It will rain.", 0)
	// Completely unrelated → high Hamming distance → miss
	_, ok := sc.Get(ctx, "machine learning neural networks deep", 0.85)
	if ok {
		t.Error("unrelated prompts should not produce a semantic hit")
	}
}

// ── Stats, Delete, TTL ────────────────────────────────────────────────────

func TestSemanticCache_Stats(t *testing.T) {
	sc := newTestCache()
	ctx := context.Background()

	sc.Set(ctx, "q1", "r1", 0)
	sc.Get(ctx, "q1", 0) // hit
	sc.Get(ctx, "q2", 0) // miss

	st := sc.Stats()
	if st.CacheHits != 1 {
		t.Errorf("hits: got %d, want 1", st.CacheHits)
	}
	if st.CacheMisses != 1 {
		t.Errorf("misses: got %d, want 1", st.CacheMisses)
	}
	if st.HitRate != 0.5 {
		t.Errorf("hit_rate: got %f, want 0.5", st.HitRate)
	}
}

func TestSemanticCache_Delete(t *testing.T) {
	sc := newTestCache()
	ctx := context.Background()
	sc.Set(ctx, "hello", "world", 0)
	if !sc.Delete("hello") {
		t.Error("Delete should return true")
	}
	_, ok := sc.Get(ctx, "hello", 0)
	if ok {
		t.Error("entry should be gone after delete")
	}
}

func TestSemanticCache_TTLExpiry(t *testing.T) {
	sc := cache.New(cache.Config{
		MaxElements:         100,
		SimilarityThreshold: 0.85,
		DefaultTTL:          50 * time.Millisecond,
	})
	ctx := context.Background()
	sc.Set(ctx, "ephemeral prompt", "data", 50*time.Millisecond)
	time.Sleep(100 * time.Millisecond)
	_, ok := sc.Get(ctx, "ephemeral prompt", 0)
	if ok {
		t.Error("entry should have expired")
	}
}
