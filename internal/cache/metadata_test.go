package cache_test

import (
	"testing"
	"time"

	"github.com/EricNguyen1206/erion-ember/internal/cache"
)

func TestMetadataSetGet(t *testing.T) {
	s := cache.NewMetadataStore(100)
	e := &cache.Entry{VectorID: 1, OriginalResponseSize: 10, CompressedResponse: []byte("data")}
	s.Set(42, e, 0)
	got := s.FindByHash(42)
	if got == nil {
		t.Fatal("expected entry, got nil")
	}
	if got.VectorID != 1 {
		t.Errorf("got VectorID %d, want 1", got.VectorID)
	}
}

func TestMetadataTTLExpiry(t *testing.T) {
	s := cache.NewMetadataStore(100)
	s.Set(99, &cache.Entry{VectorID: 99}, 50*time.Millisecond)
	time.Sleep(100 * time.Millisecond)
	if s.FindByHash(99) != nil {
		t.Error("entry should have expired")
	}
}

func TestMetadataLRUEviction(t *testing.T) {
	s := cache.NewMetadataStore(2)
	s.Set(1, &cache.Entry{VectorID: 1}, 0)
	s.Set(2, &cache.Entry{VectorID: 2}, 0)
	s.Set(3, &cache.Entry{VectorID: 3}, 0) // evicts 1
	if s.FindByHash(1) != nil {
		t.Error("key 1 should be evicted")
	}
	if s.FindByHash(2) == nil {
		t.Error("key 2 should survive")
	}
}

func TestMetadataDelete(t *testing.T) {
	s := cache.NewMetadataStore(100)
	s.Set(7, &cache.Entry{VectorID: 7}, 0)
	if !s.Delete(7) {
		t.Error("expected true")
	}
	if s.FindByHash(7) != nil {
		t.Error("should be gone")
	}
}

func TestMetadataFindByVectorID(t *testing.T) {
	s := cache.NewMetadataStore(100)
	s.Set(55, &cache.Entry{VectorID: 42}, 0)
	e := s.FindByVectorID(42)
	if e == nil {
		t.Fatal("expected to find by vectorID")
	}
}
