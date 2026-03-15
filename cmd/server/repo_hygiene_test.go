package main_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRepoDocsDoNotMentionLegacySemanticCacheTerms(t *testing.T) {
	root := repoRoot(t)
	forbidden := []string{
		"semantic",
		"vector",
		"bm25",
		"jaccard",
		"embedding",
		"similarity",
	}
	paths := []string{
		".env.example",
		"Dockerfile",
		"README.md",
		"CONTRIBUTING.md",
		"docs/API_REFERENCE.md",
		"docs/ARCHITECTURE.md",
		".github/workflows/docker-publish.yml",
	}

	for _, relPath := range paths {
		content, err := os.ReadFile(filepath.Join(root, relPath))
		if err != nil {
			t.Fatalf("read %s: %v", relPath, err)
		}

		text := strings.ToLower(string(content))
		for _, token := range forbidden {
			if strings.Contains(text, token) {
				t.Fatalf("%s still contains legacy token %q", relPath, token)
			}
		}
	}
}
