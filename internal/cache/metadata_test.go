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
	s.Set(1, &cache.Entry{PromptHash: 1, SimHash: 0xABC}, 0)
	s.Set(2, &cache.Entry{PromptHash: 2, SimHash: 0xDEF}, 0)
	all := s.ScanAll()
	if len(all) != 2 {
		t.Errorf("ScanAll: got %d entries, want 2", len(all))
	}
}
