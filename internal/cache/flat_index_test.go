package cache_test

import (
	"errors"
	"testing"

	"github.com/EricNguyen1206/erion-ember/internal/cache"
)

func TestFlatIndexSearchReturnsNearestNeighbor(t *testing.T) {
	index := cache.NewFlatIndex()
	namespaceKey := cache.NamespaceKey(cache.Namespace{
		Model:            "text-embedding-3-large",
		TenantID:         "tenant-a",
		SystemPromptHash: "sys-a",
	})

	if err := index.Insert(&cache.Entry{ID: "near", NamespaceKey: namespaceKey, PromptHash: 1, Vector: []float32{1, 0}}); err != nil {
		t.Fatalf("Insert() error = %v, want nil", err)
	}
	if err := index.Insert(&cache.Entry{ID: "far", NamespaceKey: namespaceKey, PromptHash: 2, Vector: []float32{0, 1}}); err != nil {
		t.Fatalf("Insert() error = %v, want nil", err)
	}

	results := index.Search(namespaceKey, []float32{0.9, 0.1}, 1)
	if len(results) != 1 {
		t.Fatalf("len(Search()) = %d, want 1", len(results))
	}

	if results[0].EntryID != "near" {
		t.Fatalf("Search()[0].EntryID = %q, want %q", results[0].EntryID, "near")
	}

	if results[0].PromptHash != 1 {
		t.Fatalf("Search()[0].PromptHash = %d, want 1", results[0].PromptHash)
	}

	if results[0].Score <= 0 {
		t.Fatalf("Search()[0].Score = %f, want positive cosine similarity", results[0].Score)
	}
}

func TestFlatIndexSearchScopesResultsToNamespace(t *testing.T) {
	index := cache.NewFlatIndex()
	namespaceA := cache.NamespaceKey(cache.Namespace{Model: "text-embedding-3-large", TenantID: "tenant-a", SystemPromptHash: "sys-a"})
	namespaceB := cache.NamespaceKey(cache.Namespace{Model: "text-embedding-3-large", TenantID: "tenant-b", SystemPromptHash: "sys-a"})

	if err := index.Insert(&cache.Entry{ID: "a", NamespaceKey: namespaceA, PromptHash: 1, Vector: []float32{1, 0}}); err != nil {
		t.Fatalf("Insert() error = %v, want nil", err)
	}
	if err := index.Insert(&cache.Entry{ID: "b", NamespaceKey: namespaceB, PromptHash: 2, Vector: []float32{1, 0}}); err != nil {
		t.Fatalf("Insert() error = %v, want nil", err)
	}

	results := index.Search(namespaceA, []float32{1, 0}, 10)
	if len(results) != 1 {
		t.Fatalf("len(Search()) = %d, want 1", len(results))
	}
	if results[0].NamespaceKey != namespaceA {
		t.Fatalf("Search()[0].NamespaceKey = %q, want %q", results[0].NamespaceKey, namespaceA)
	}
}

func TestFlatIndexDeleteAndStats(t *testing.T) {
	index := cache.NewFlatIndex()
	namespaceKey := cache.NamespaceKey(cache.Namespace{Model: "text-embedding-3-large", TenantID: "tenant-a", SystemPromptHash: "sys-a"})

	if err := index.Insert(&cache.Entry{ID: "first", NamespaceKey: namespaceKey, PromptHash: 1, Vector: []float32{1, 0, 0}}); err != nil {
		t.Fatalf("Insert() error = %v, want nil", err)
	}
	if err := index.Insert(&cache.Entry{ID: "second", NamespaceKey: namespaceKey, PromptHash: 2, Vector: []float32{0, 1, 0}}); err != nil {
		t.Fatalf("Insert() error = %v, want nil", err)
	}

	stats := index.Stats()
	if stats.Namespaces != 1 || stats.Vectors != 2 || stats.Dimension != 3 {
		t.Fatalf("Stats() = %+v, want namespaces=1 vectors=2 dimension=3", stats)
	}

	if !index.Delete(namespaceKey, 1) {
		t.Fatal("Delete() = false, want true")
	}

	results := index.Search(namespaceKey, []float32{1, 0, 0}, 10)
	if len(results) != 1 {
		t.Fatalf("len(Search()) after delete = %d, want 1", len(results))
	}
	if results[0].EntryID != "second" {
		t.Fatalf("Search()[0].EntryID after delete = %q, want %q", results[0].EntryID, "second")
	}
}

func TestFlatIndexInsertRejectsDimensionMismatch(t *testing.T) {
	index := cache.NewFlatIndex()
	namespaceKey := cache.NamespaceKey(cache.Namespace{Model: "text-embedding-3-large", TenantID: "tenant-a", SystemPromptHash: "sys-a"})

	if err := index.Insert(&cache.Entry{ID: "valid", NamespaceKey: namespaceKey, PromptHash: 1, Vector: []float32{1, 0}}); err != nil {
		t.Fatalf("first Insert() error = %v, want nil", err)
	}

	err := index.Insert(&cache.Entry{ID: "invalid", NamespaceKey: namespaceKey, PromptHash: 2, Vector: []float32{1, 0, 0}})
	if err == nil {
		t.Fatal("second Insert() error = nil, want dimension mismatch error")
	}
	if !errors.Is(err, cache.ErrVectorDimensionMismatch) {
		t.Fatalf("second Insert() error = %v, want ErrVectorDimensionMismatch", err)
	}

	stats := index.Stats()
	if stats.Namespaces != 1 || stats.Vectors != 1 || stats.Dimension != 2 {
		t.Fatalf("Stats() after rejected insert = %+v, want namespaces=1 vectors=1 dimension=2", stats)
	}

	results := index.Search(namespaceKey, []float32{1, 0}, 10)
	if len(results) != 1 {
		t.Fatalf("len(Search()) after rejected insert = %d, want 1", len(results))
	}
	if results[0].EntryID != "valid" {
		t.Fatalf("Search()[0].EntryID after rejected insert = %q, want %q", results[0].EntryID, "valid")
	}
}

func TestFlatIndexInsertAllowsDifferentDimensionsAcrossNamespaces(t *testing.T) {
	index := cache.NewFlatIndex()
	namespaceA := cache.NamespaceKey(cache.Namespace{Model: "text-embedding-3-large", TenantID: "tenant-a", SystemPromptHash: "sys-a"})
	namespaceB := cache.NamespaceKey(cache.Namespace{Model: "text-embedding-3-small", TenantID: "tenant-b", SystemPromptHash: "sys-b"})

	if err := index.Insert(&cache.Entry{ID: "a", NamespaceKey: namespaceA, PromptHash: 1, Vector: []float32{1, 0}}); err != nil {
		t.Fatalf("Insert() namespace A error = %v, want nil", err)
	}
	if err := index.Insert(&cache.Entry{ID: "b", NamespaceKey: namespaceB, PromptHash: 2, Vector: []float32{1, 0, 0}}); err != nil {
		t.Fatalf("Insert() namespace B error = %v, want nil", err)
	}

	resultsA := index.Search(namespaceA, []float32{1, 0}, 10)
	if len(resultsA) != 1 || resultsA[0].EntryID != "a" {
		t.Fatalf("Search() namespace A = %+v, want one result for entry a", resultsA)
	}

	resultsB := index.Search(namespaceB, []float32{1, 0, 0}, 10)
	if len(resultsB) != 1 || resultsB[0].EntryID != "b" {
		t.Fatalf("Search() namespace B = %+v, want one result for entry b", resultsB)
	}

	stats := index.Stats()
	if stats.Namespaces != 2 || stats.Vectors != 2 {
		t.Fatalf("Stats() = %+v, want namespaces=2 vectors=2", stats)
	}
}

func TestFlatIndexDeleteLastEntryResetsNamespaceDimension(t *testing.T) {
	index := cache.NewFlatIndex()
	namespaceKey := cache.NamespaceKey(cache.Namespace{Model: "text-embedding-3-large", TenantID: "tenant-a", SystemPromptHash: "sys-a"})

	if err := index.Insert(&cache.Entry{ID: "first", NamespaceKey: namespaceKey, PromptHash: 1, Vector: []float32{1, 0}}); err != nil {
		t.Fatalf("first Insert() error = %v, want nil", err)
	}
	if !index.Delete(namespaceKey, 1) {
		t.Fatal("Delete() = false, want true")
	}

	stats := index.Stats()
	if stats.Namespaces != 0 || stats.Vectors != 0 || stats.Dimension != 0 {
		t.Fatalf("Stats() after empty namespace = %+v, want namespaces=0 vectors=0 dimension=0", stats)
	}

	if err := index.Insert(&cache.Entry{ID: "second", NamespaceKey: namespaceKey, PromptHash: 2, Vector: []float32{1, 0, 0}}); err != nil {
		t.Fatalf("second Insert() error = %v, want nil after namespace reset", err)
	}

	results := index.Search(namespaceKey, []float32{1, 0, 0}, 10)
	if len(results) != 1 || results[0].EntryID != "second" {
		t.Fatalf("Search() after namespace reset = %+v, want one result for entry second", results)
	}

	stats = index.Stats()
	if stats.Namespaces != 1 || stats.Vectors != 1 || stats.Dimension != 3 {
		t.Fatalf("Stats() after namespace reset insert = %+v, want namespaces=1 vectors=1 dimension=3", stats)
	}
}
