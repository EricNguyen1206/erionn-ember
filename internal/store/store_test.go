package store

import (
	"testing"
	"time"
)

func TestStoreDeleteTypeExpireTTLAndStats(t *testing.T) {
	s := New()
	s.entries["alpha"] = &Entry{Key: "alpha", Type: TypeString, Value: "one"}

	if !s.Exists("alpha") {
		t.Fatal("expected alpha to exist")
	}
	if got := s.Type("alpha"); got != TypeString {
		t.Fatalf("got type %q, want %q", got, TypeString)
	}
	if !s.Expire("alpha", time.Second) {
		t.Fatal("expected ttl to be set")
	}
	ttl, hasTTL, found := s.TTL("alpha")
	if !found || !hasTTL || ttl <= 0 {
		t.Fatalf("unexpected ttl state: ttl=%v hasTTL=%v found=%v", ttl, hasTTL, found)
	}
	if !s.Del("alpha") {
		t.Fatal("expected delete success")
	}
	if s.Stats().TotalKeys != 0 {
		t.Fatal("expected empty store after delete")
	}
}

func TestStoreExpireDoesNotRewriteCreatedAt(t *testing.T) {
	s := New()
	s.entries["alpha"] = &Entry{Key: "alpha", Type: TypeString, Value: "one"}

	if !s.Expire("alpha", time.Second) {
		t.Fatal("expected ttl update")
	}
	if !s.entries["alpha"].CreatedAt.IsZero() {
		t.Fatal("expected Expire to leave CreatedAt unchanged")
	}
}

func TestStoreTTLReportsMissingAndNoExpiry(t *testing.T) {
	s := New()

	if _, hasTTL, found := s.TTL("missing"); found || hasTTL {
		t.Fatal("expected missing key to report found=false and hasTTL=false")
	}

	s.entries["alpha"] = &Entry{Key: "alpha", Type: TypeString, Value: "one"}
	if ttl, hasTTL, found := s.TTL("alpha"); !found || hasTTL || ttl != 0 {
		t.Fatalf("unexpected ttl state: ttl=%v hasTTL=%v found=%v", ttl, hasTTL, found)
	}
}

func TestStoreStatsPruneExpiredAndCountTypes(t *testing.T) {
	s := New()
	s.entries["string"] = &Entry{Key: "string", Type: TypeString, Value: "one"}
	s.entries["hash"] = &Entry{Key: "hash", Type: TypeHash, Value: map[string]string{"name": "eric"}}
	s.entries["list"] = &Entry{Key: "list", Type: TypeList, Value: []string{"a", "b"}}
	s.entries["set"] = &Entry{Key: "set", Type: TypeSet, Value: map[string]struct{}{"go": {}}}
	s.entries["expired"] = &Entry{Key: "expired", Type: TypeString, Value: "old", ExpiresAt: time.Now().Add(-time.Second)}

	stats := s.Stats()
	if stats.TotalKeys != 4 || stats.StringKeys != 1 || stats.HashKeys != 1 || stats.ListKeys != 1 || stats.SetKeys != 1 {
		t.Fatalf("unexpected stats: %+v", stats)
	}
	if _, ok := s.entries["expired"]; ok {
		t.Fatal("expected Stats to prune expired key")
	}
}
