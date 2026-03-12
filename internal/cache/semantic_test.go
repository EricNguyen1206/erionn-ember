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

func TestSemanticCache_ExactHit(t *testing.T) {
	sc := newTestCache()
	ctx := context.Background()

	if _, err := sc.Set(ctx, "hello world", "how are you?", 0); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	result, ok := sc.Get(ctx, "hello world", 0.85)
	if !ok {
		t.Fatal("expected hit, got miss")
	}
	if !result.ExactMatch {
		t.Error("expected exact match")
	}
	if result.Response != "how are you?" {
		t.Errorf("wrong response: %q", result.Response)
	}
	if result.Similarity != 1.0 {
		t.Errorf("wrong similarity: %f, want 1.0", result.Similarity)
	}
}

func TestSemanticCache_ExactMiss(t *testing.T) {
	sc := newTestCache()
	ctx := context.Background()

	if _, err := sc.Set(ctx, "hello world", "how are you?", 0); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	_, ok := sc.Get(ctx, "hello universe", 0.85)
	if ok {
		t.Fatal("expected miss, got hit")
	}
}

func TestSemanticCache_NormalizeBeforeHash(t *testing.T) {
	sc := newTestCache()
	ctx := context.Background()

	if _, err := sc.Set(ctx, "  Hello   WORLD  ", "how are you?", 0); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// "hello world" hashes to same value as normalized "  Hello   WORLD!  "
	result, ok := sc.Get(ctx, "hello world", 0.85)
	if !ok {
		t.Fatal("expected hit, got miss")
	}
	if !result.ExactMatch {
		t.Error("expected exact match")
	}
}

// ── BM25+Jaccard semantic similarity (slow path) ─────────────────────────

func TestSemanticCache_SimilarityHit_SameTokens(t *testing.T) {
	// BM25+Jaccard is order-independent: same token set → Jaccard=1.0
	// "go language fast compiled" and "compiled fast language go" share all tokens.
	// With a single-doc corpus, combined score ~0.67 → use threshold 0.6.
	sc := newTestCache()
	ctx := context.Background()

	if _, err := sc.Set(ctx, "go language fast compiled", "Cached response.", 0); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Different word order → different xxhash (exact miss) but Jaccard=1.0 (semantic hit)
	result, ok := sc.Get(ctx, "compiled fast language go", 0.6)
	if !ok {
		t.Fatal("expected BM25+Jaccard similarity hit for same-token rearrangement")
	}
	if result.ExactMatch {
		t.Error("different word order should not be an exact match")
	}
	if result.Response != "Cached response." {
		t.Errorf("wrong response: %q", result.Response)
	}
	if result.Similarity < 0.6 {
		t.Errorf("similarity %f should be ≥ 0.6", result.Similarity)
	}
}

func TestSemanticCache_SimilarityHit_Paraphrase(t *testing.T) {
	// BM25+Jaccard advantage over SimHash: partial token overlap
	// "explain goroutines in go" vs "how do goroutines work in go" share key tokens.
	sc := newTestCache()
	ctx := context.Background()
	if _, err := sc.Set(ctx, "explain goroutines in go", "Goroutines are lightweight threads.", 0); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// With a single-doc corpus, partial overlap score is around 0.22.
	// We use 0.2 as a threshold for detection in this minimal set.
	result, ok := sc.Get(ctx, "how do goroutines work in go", 0.2)
	if !ok {
		t.Fatal("expected paraphrase hit — goroutines+go in common")
	}
	if result.Response != "Goroutines are lightweight threads." {
		t.Errorf("wrong response: %q", result.Response)
	}
	if result.Similarity < 0.2 {
		t.Errorf("similarity %f should be ≥ 0.2", result.Similarity)
	}
}

func TestSemanticCache_SimHashMiss_Unrelated(t *testing.T) {
	sc := newTestCache()
	ctx := context.Background()
	if _, err := sc.Set(ctx, "weather forecast tomorrow rain", "It will rain.", 0); err != nil {
		t.Fatalf("Set failed: %v", err)
	}
	// Completely unrelated → zero token overlap → Jaccard=0, BM25=0 → miss
	_, ok := sc.Get(ctx, "machine learning neural networks deep", 0.85)
	if ok {
		t.Error("unrelated prompts should not produce a semantic hit")
	}
}

func TestSemanticCache_Stats(t *testing.T) {
	sc := newTestCache()
	ctx := context.Background()

	if _, err := sc.Set(ctx, "one", "un", 0); err != nil {
		t.Fatalf("Set failed: %v", err)
	}
	if _, err := sc.Set(ctx, "two", "deux", 0); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	sc.Get(ctx, "one", 0)   // hit
	sc.Get(ctx, "three", 0) // miss

	stats := sc.Stats()
	if stats.TotalEntries != 2 {
		t.Errorf("got %d entries, want 2", stats.TotalEntries)
	}
	if stats.CacheHits != 1 {
		t.Errorf("got %d hits, want 1", stats.CacheHits)
	}
	if stats.CacheMisses != 1 {
		t.Errorf("got %d misses, want 1", stats.CacheMisses)
	}
	if stats.TotalQueries != 2 {
		t.Errorf("got %d total, want 2", stats.TotalQueries)
	}
	if stats.HitRate != 0.5 {
		t.Errorf("got hit rate %f, want 0.5", stats.HitRate)
	}
}

func TestSemanticCache_Delete(t *testing.T) {
	sc := newTestCache()
	ctx := context.Background()

	if _, err := sc.Set(ctx, "delete me", "done", 0); err != nil {
		t.Fatalf("Set failed: %v", err)
	}
	if !sc.Delete("delete me") {
		t.Fatal("Delete failed")
	}

	_, ok := sc.Get(ctx, "delete me", 0)
	if ok {
		t.Fatal("expected miss after delete")
	}
}

func TestSemanticCache_TTLExpiry(t *testing.T) {
	sc := newTestCache()
	ctx := context.Background()

	if _, err := sc.Set(ctx, "expire me", "bye", 50*time.Millisecond); err != nil {
		t.Fatalf("Set failed: %v", err)
	}
	time.Sleep(100 * time.Millisecond)

	_, ok := sc.Get(ctx, "expire me", 0)
	if ok {
		t.Fatal("expected miss after TTL expiry")
	}
}

func TestSemanticCache_StatsExcludeExpiredEntries(t *testing.T) {
	sc := newTestCache()
	ctx := context.Background()

	if _, err := sc.Set(ctx, "expire soon", "bye", 50*time.Millisecond); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	stats := sc.Stats()
	if stats.TotalEntries != 0 {
		t.Fatalf("got %d entries, want 0 after expiry", stats.TotalEntries)
	}
}

func TestSemanticCache_SetReplacesExistingPrompt(t *testing.T) {
	sc := newTestCache()
	ctx := context.Background()

	if _, err := sc.Set(ctx, "hello world", "first", 0); err != nil {
		t.Fatalf("first Set failed: %v", err)
	}
	if _, err := sc.Set(ctx, "hello world", "second", 0); err != nil {
		t.Fatalf("second Set failed: %v", err)
	}

	result, ok := sc.Get(ctx, "hello world", 0)
	if !ok {
		t.Fatal("expected hit, got miss")
	}
	if result.Response != "second" {
		t.Fatalf("got response %q, want %q", result.Response, "second")
	}

	stats := sc.Stats()
	if stats.TotalEntries != 1 {
		t.Fatalf("got %d entries, want 1", stats.TotalEntries)
	}
}

func BenchmarkSemanticCache_Set(b *testing.B) {
	sc := cache.New(cache.Config{MaxElements: 1000})
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := sc.Set(ctx, "how to use semantic cache in go", "You can use erion-ember.", 0); err != nil {
			b.Fatalf("Set failed: %v", err)
		}
	}
}

func BenchmarkSemanticCache_GetHit(b *testing.B) {
	sc := cache.New(cache.Config{MaxElements: 1000})
	ctx := context.Background()
	prompt := "how to use semantic cache in go"
	if _, err := sc.Set(ctx, prompt, "You can use erion-ember.", 0); err != nil {
		b.Fatalf("Set failed: %v", err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = sc.Get(ctx, prompt, 0.85)
	}
}

func BenchmarkSemanticCache_GetMiss(b *testing.B) {
	sc := cache.New(cache.Config{MaxElements: 1000})
	ctx := context.Background()
	if _, err := sc.Set(ctx, "something else", "response", 0); err != nil {
		b.Fatalf("Set failed: %v", err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = sc.Get(ctx, "how to use semantic cache in go", 0.85)
	}
}
