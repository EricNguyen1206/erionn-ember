package cache_test

import (
	"testing"

	"github.com/EricNguyen1206/erion-ember/internal/cache"
)

func TestNamespaceKeyStable(t *testing.T) {
	ns := cache.Namespace{
		Model:            "text-embedding-3-large",
		TenantID:         "tenant-a",
		SystemPromptHash: "system-hash",
	}

	got := cache.NamespaceKey(ns)
	want := `["text-embedding-3-large","tenant-a","system-hash"]`
	if got != want {
		t.Fatalf("NamespaceKey() = %q, want %q", got, want)
	}

	if gotAgain := cache.NamespaceKey(ns); gotAgain != want {
		t.Fatalf("NamespaceKey() second call = %q, want %q", gotAgain, want)
	}

	changed := ns
	changed.TenantID = "tenant-b"
	if cache.NamespaceKey(changed) == want {
		t.Fatal("NamespaceKey() should change when namespace fields change")
	}
}

func TestNamespaceKeyCollisionSafe(t *testing.T) {
	left := cache.Namespace{
		Model:            "a:b",
		TenantID:         "c",
		SystemPromptHash: "d",
	}
	right := cache.Namespace{
		Model:            "a",
		TenantID:         "b:c",
		SystemPromptHash: "d",
	}

	leftKey := cache.NamespaceKey(left)
	rightKey := cache.NamespaceKey(right)
	if leftKey == rightKey {
		t.Fatalf("NamespaceKey() collision: %q", leftKey)
	}
	if leftKey != `["a:b","c","d"]` {
		t.Fatalf("left NamespaceKey() = %q", leftKey)
	}
	if rightKey != `["a","b:c","d"]` {
		t.Fatalf("right NamespaceKey() = %q", rightKey)
	}
}
