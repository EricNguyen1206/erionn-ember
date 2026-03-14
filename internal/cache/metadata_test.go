package cache_test

import (
	"testing"
	"time"

	"github.com/EricNguyen1206/erion-ember/internal/cache"
)

func TestMetadataSetGet(t *testing.T) {
	s := cache.NewMetadataStore(100)
	e := &cache.Entry{PromptHash: 42, OriginalResponseSize: 10, CompressedResponse: []byte("data")}
	s.Set(42, e, 0)
	got := s.FindByHash(42)
	if got == nil {
		t.Fatal("expected entry, got nil")
	}
}

func TestMetadataTTLExpiry(t *testing.T) {
	s := cache.NewMetadataStore(100)
	s.Set(99, &cache.Entry{PromptHash: 99}, 50*time.Millisecond)
	time.Sleep(100 * time.Millisecond)
	if s.FindByHash(99) != nil {
		t.Error("entry should have expired")
	}
}

func TestMetadataLRUEviction(t *testing.T) {
	s := cache.NewMetadataStore(2)
	s.Set(1, &cache.Entry{PromptHash: 1}, 0)
	s.Set(2, &cache.Entry{PromptHash: 2}, 0)
	s.Set(3, &cache.Entry{PromptHash: 3}, 0) // evicts 1
	if s.FindByHash(1) != nil {
		t.Error("key 1 should be evicted")
	}
	if s.FindByHash(2) == nil {
		t.Error("key 2 should survive")
	}
}

func TestMetadataDelete(t *testing.T) {
	s := cache.NewMetadataStore(100)
	s.Set(7, &cache.Entry{PromptHash: 7}, 0)
	if !s.Delete(7) {
		t.Error("expected true")
	}
	if s.FindByHash(7) != nil {
		t.Error("should be gone")
	}
}

func TestMetadataScanAll(t *testing.T) {
	s := cache.NewMetadataStore(100)
	s.Set(1, &cache.Entry{PromptHash: 1, Tokens: []string{"hello"}}, 0)
	s.Set(2, &cache.Entry{PromptHash: 2, Tokens: []string{"world"}}, 0)
	all := s.ScanAll()
	if len(all) != 2 {
		t.Errorf("ScanAll: got %d entries, want 2", len(all))
	}
}

func TestMetadataStoreSeparatesNamespaces(t *testing.T) {
	s := cache.NewMetadataStore(100)
	hash := uint64(42)
	nsA := cache.NamespaceKey(cache.Namespace{Model: "gpt-4o", TenantID: "tenant-a", SystemPromptHash: "sys-a"})
	nsB := cache.NamespaceKey(cache.Namespace{Model: "gpt-4o", TenantID: "tenant-b", SystemPromptHash: "sys-a"})

	entryA := &cache.Entry{CompressedResponse: []byte("a"), Vector: []float32{0.1, 0.2}}
	entryB := &cache.Entry{CompressedResponse: []byte("b"), Vector: []float32{0.3, 0.4}}

	s.SetExact(nsA, hash, entryA, 0)
	s.SetExact(nsB, hash, entryB, 0)

	gotA := s.FindExactByHash(nsA, hash)
	if gotA == nil {
		t.Fatal("expected namespace A entry, got nil")
	}
	if string(gotA.CompressedResponse) != "a" {
		t.Fatalf("namespace A response = %q, want %q", gotA.CompressedResponse, "a")
	}
	if gotA.NamespaceKey != nsA {
		t.Fatalf("namespace A key = %q, want %q", gotA.NamespaceKey, nsA)
	}
	if len(gotA.Vector) != 2 || gotA.Vector[0] != 0.1 || gotA.Vector[1] != 0.2 {
		t.Fatalf("namespace A vector = %v, want [0.1 0.2]", gotA.Vector)
	}

	gotB := s.FindExactByHash(nsB, hash)
	if gotB == nil {
		t.Fatal("expected namespace B entry, got nil")
	}
	if string(gotB.CompressedResponse) != "b" {
		t.Fatalf("namespace B response = %q, want %q", gotB.CompressedResponse, "b")
	}
	if gotB.NamespaceKey != nsB {
		t.Fatalf("namespace B key = %q, want %q", gotB.NamespaceKey, nsB)
	}
	if len(gotB.Vector) != 2 || gotB.Vector[0] != 0.3 || gotB.Vector[1] != 0.4 {
		t.Fatalf("namespace B vector = %v, want [0.3 0.4]", gotB.Vector)
	}
}

func TestMetadataStoreLegacyNamespaceIsolationOnSameHash(t *testing.T) {
	s := cache.NewMetadataStore(100)
	hash := uint64(77)
	ns := cache.NamespaceKey(cache.Namespace{Model: "text-embedding-3-large", TenantID: "tenant-a", SystemPromptHash: "sys-a"})

	legacy := &cache.Entry{CompressedResponse: []byte("legacy")}
	namespaced := &cache.Entry{CompressedResponse: []byte("namespaced"), Vector: []float32{0.5, 0.6}}

	s.Set(hash, legacy, 0)
	s.SetExact(ns, hash, namespaced, 0)

	legacyGot := s.FindByHash(hash)
	if legacyGot == nil {
		t.Fatal("expected legacy entry, got nil")
	}
	if string(legacyGot.CompressedResponse) != "legacy" {
		t.Fatalf("legacy response = %q, want %q", legacyGot.CompressedResponse, "legacy")
	}
	if legacyGot.NamespaceKey != "" {
		t.Fatalf("legacy namespace key = %q, want empty", legacyGot.NamespaceKey)
	}

	namespacedGot := s.FindExactByHash(ns, hash)
	if namespacedGot == nil {
		t.Fatal("expected namespaced entry, got nil")
	}
	if string(namespacedGot.CompressedResponse) != "namespaced" {
		t.Fatalf("namespaced response = %q, want %q", namespacedGot.CompressedResponse, "namespaced")
	}
	if namespacedGot.NamespaceKey != ns {
		t.Fatalf("namespaced namespace key = %q, want %q", namespacedGot.NamespaceKey, ns)
	}

	if !s.Delete(hash) {
		t.Fatal("expected legacy delete to succeed")
	}
	if s.FindByHash(hash) != nil {
		t.Fatal("expected legacy entry to be deleted")
	}
	remaining := s.FindExactByHash(ns, hash)
	if remaining == nil {
		t.Fatal("expected namespaced entry to remain after legacy delete")
	}
	if string(remaining.CompressedResponse) != "namespaced" {
		t.Fatalf("remaining namespaced response = %q, want %q", remaining.CompressedResponse, "namespaced")
	}
}
