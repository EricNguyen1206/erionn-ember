package cache_test

import (
	"context"
	"fmt"
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

type fakeEmbedder struct {
	vectors map[string][]float32
}

func (e fakeEmbedder) Embed(_ context.Context, text string) ([]float32, error) {
	vector, ok := e.vectors[text]
	if !ok {
		return nil, fmt.Errorf("unexpected embed request for %q", text)
	}
	return append([]float32(nil), vector...), nil
}

func (e fakeEmbedder) Dimension() int {
	for _, vector := range e.vectors {
		return len(vector)
	}
	return 0
}

func newVectorTestCache(vectors map[string][]float32) *cache.SemanticCache {
	return cache.NewWithDependencies(
		cache.Config{
			MaxElements:         100,
			SimilarityThreshold: 0.85,
			DefaultTTL:          time.Hour,
		},
		fakeEmbedder{vectors: vectors},
		cache.NewFlatIndex(),
	)
}

func newBenchmarkVectorTestCache(maxElements int, vectors map[string][]float32) *cache.SemanticCache {
	return cache.NewWithDependencies(
		cache.Config{
			MaxElements:         maxElements,
			SimilarityThreshold: 0.85,
			DefaultTTL:          time.Hour,
		},
		fakeEmbedder{vectors: vectors},
		cache.NewFlatIndex(),
	)
}

func benchmarkSemanticCorpus(size int) (map[string][]float32, []string, string, string) {
	vectors := make(map[string][]float32, size+1)
	prompts := make([]string, 0, size)
	semanticTarget := fmt.Sprintf("stored prompt %03d", size/2)
	for i := 0; i < size; i++ {
		prompt := fmt.Sprintf("stored prompt %03d", i)
		vector := []float32{0, 1, float32((i % 17) + 1)}
		if prompt == semanticTarget {
			vector = []float32{1, 0, 0}
		}
		vectors[prompt] = vector
		prompts = append(prompts, prompt)
	}

	queryPrompt := "semantic lookup query"
	vectors[queryPrompt] = append([]float32(nil), vectors[semanticTarget]...)

	return vectors, prompts, queryPrompt, semanticTarget
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

func TestSemanticCache_NamespaceIsolation(t *testing.T) {
	sc := newVectorTestCache(map[string][]float32{
		"tenant a prompt": {1, 0},
		"tenant b prompt": {1, 0},
		"shared query":    {1, 0},
	})
	ctx := context.Background()
	nsA := cache.Namespace{Model: "text-embedding-3-small", TenantID: "tenant-a", SystemPromptHash: "sys"}
	nsB := cache.Namespace{Model: "text-embedding-3-small", TenantID: "tenant-b", SystemPromptHash: "sys"}

	if _, err := sc.SetInNamespace(ctx, nsA, "tenant a prompt", "response-a", 0); err != nil {
		t.Fatalf("SetInNamespace() tenant A failed: %v", err)
	}
	if _, err := sc.SetInNamespace(ctx, nsB, "tenant b prompt", "response-b", 0); err != nil {
		t.Fatalf("SetInNamespace() tenant B failed: %v", err)
	}

	resultA, ok := sc.GetInNamespace(ctx, nsA, "shared query", 0.8)
	if !ok {
		t.Fatal("expected namespace A semantic hit, got miss")
	}
	if resultA.Response != "response-a" {
		t.Fatalf("namespace A response = %q, want %q", resultA.Response, "response-a")
	}

	resultB, ok := sc.GetInNamespace(ctx, nsB, "shared query", 0.8)
	if !ok {
		t.Fatal("expected namespace B semantic hit, got miss")
	}
	if resultB.Response != "response-b" {
		t.Fatalf("namespace B response = %q, want %q", resultB.Response, "response-b")
	}
}

func TestSemanticCache_ExactHitPrecedesVectorSearch(t *testing.T) {
	sc := newVectorTestCache(map[string][]float32{
		"exact prompt":  {1, 0},
		"semantic peer": {1, 0},
	})
	ctx := context.Background()

	if _, err := sc.SetInNamespace(ctx, cache.Namespace{}, "exact prompt", "exact-response", 0); err != nil {
		t.Fatalf("SetInNamespace() exact failed: %v", err)
	}
	if _, err := sc.SetInNamespace(ctx, cache.Namespace{}, "semantic peer", "semantic-response", 0); err != nil {
		t.Fatalf("SetInNamespace() semantic failed: %v", err)
	}

	result, ok := sc.Get(ctx, "exact prompt", 0.8)
	if !ok {
		t.Fatal("expected exact hit, got miss")
	}
	if !result.ExactMatch {
		t.Fatal("expected exact hit to win before vector search")
	}
	if result.Response != "exact-response" {
		t.Fatalf("response = %q, want %q", result.Response, "exact-response")
	}
}

func TestSemanticCache_VectorThreshold(t *testing.T) {
	sc := newVectorTestCache(map[string][]float32{
		"stored prompt": {0.8, 0.6},
		"query prompt":  {1, 0},
	})
	ctx := context.Background()

	if _, err := sc.Set(ctx, "stored prompt", "threshold-response", 0); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	if _, ok := sc.Get(ctx, "query prompt", 0.81); ok {
		t.Fatal("expected miss above cosine threshold")
	}

	result, ok := sc.Get(ctx, "query prompt", 0.8)
	if !ok {
		t.Fatal("expected hit at cosine threshold")
	}
	if result.ExactMatch {
		t.Fatal("expected vector hit, got exact match")
	}
	if result.Response != "threshold-response" {
		t.Fatalf("response = %q, want %q", result.Response, "threshold-response")
	}
	if result.Similarity != 0.8 {
		t.Fatalf("similarity = %f, want %f", result.Similarity, 0.8)
	}
}

func TestSemanticCache_DeleteRemovesVectorReachability(t *testing.T) {
	sc := newVectorTestCache(map[string][]float32{
		"delete me":    {1, 0},
		"nearby query": {1, 0},
	})
	ctx := context.Background()

	if _, err := sc.Set(ctx, "delete me", "gone", 0); err != nil {
		t.Fatalf("Set failed: %v", err)
	}
	if !sc.Delete("delete me") {
		t.Fatal("Delete failed")
	}

	if _, ok := sc.Get(ctx, "nearby query", 0.8); ok {
		t.Fatal("expected miss after delete removed vector reachability")
	}
}

func TestSemanticCache_TTLSkipsExpiredVectorEntries(t *testing.T) {
	sc := newVectorTestCache(map[string][]float32{
		"expire me":    {1, 0},
		"nearby query": {1, 0},
	})
	ctx := context.Background()

	if _, err := sc.Set(ctx, "expire me", "bye", 50*time.Millisecond); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	if _, ok := sc.Get(ctx, "nearby query", 0.8); ok {
		t.Fatal("expected miss after TTL expiry removed vector candidate")
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

func BenchmarkSemanticCache_GetExactHit(b *testing.B) {
	ctx := context.Background()
	namespace := cache.Namespace{Model: "text-embedding-3-small", TenantID: "tenant-a", SystemPromptHash: "sys"}
	sc := newVectorTestCache(map[string][]float32{
		"exact prompt": {1, 0},
	})
	if _, err := sc.SetInNamespace(ctx, namespace, "exact prompt", "exact-response", 0); err != nil {
		b.Fatalf("SetInNamespace failed: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result, ok := sc.GetInNamespace(ctx, namespace, "exact prompt", 0.8)
		if !ok {
			b.Fatal("expected hit, got miss")
		}
		if !result.ExactMatch {
			b.Fatal("expected exact hit")
		}
	}
}

// BenchmarkSemanticCache_GetVectorSemanticHit measures cache-side semantic lookup
// overhead using a fake embedder so embedder runtime does not dominate the result.
func BenchmarkSemanticCache_GetVectorSemanticHit(b *testing.B) {
	ctx := context.Background()
	namespace := cache.Namespace{Model: "text-embedding-3-small", TenantID: "tenant-a", SystemPromptHash: "sys"}
	vectors, prompts, queryPrompt, semanticTarget := benchmarkSemanticCorpus(256)
	sc := newBenchmarkVectorTestCache(len(prompts)+1, vectors)
	for _, prompt := range prompts {
		response := fmt.Sprintf("response for %s", prompt)
		if prompt == semanticTarget {
			response = "semantic-response"
		}
		if _, err := sc.SetInNamespace(ctx, namespace, prompt, response, 0); err != nil {
			b.Fatalf("SetInNamespace(%q) failed: %v", prompt, err)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result, ok := sc.GetInNamespace(ctx, namespace, queryPrompt, 0.8)
		if !ok {
			b.Fatal("expected semantic hit, got miss")
		}
		if result.ExactMatch {
			b.Fatal("expected vector semantic hit")
		}
		if result.Response != "semantic-response" {
			b.Fatalf("response = %q, want %q", result.Response, "semantic-response")
		}
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
