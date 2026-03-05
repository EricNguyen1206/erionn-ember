package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// OllamaEmbedder calls Ollama's embedding API over HTTP.
// Pure Go — zero CGO, zero C libraries, no install required (beyond running Ollama).
//
// Setup:
//  1. Install Ollama:  https://ollama.com/download
//  2. Pull a model:    ollama pull nomic-embed-text
//  3. Set env vars:    OLLAMA_URL=http://localhost:11434  OLLAMA_MODEL=nomic-embed-text
//
// Model recommendations (embedding-focused):
//
//	nomic-embed-text      — 768-dim, fast, great quality (~270MB)
//	mxbai-embed-large     — 1024-dim, best quality (~670MB)
//	all-minilm            — 384-dim, very fast, lightweight (~45MB)
type OllamaEmbedder struct {
	url    string // e.g. "http://localhost:11434"
	model  string // e.g. "nomic-embed-text"
	client *http.Client
	dim    int
}

// NewOllamaEmbedder creates an OllamaEmbedder and probes the dimension.
func NewOllamaEmbedder(ollamaURL, model string) (*OllamaEmbedder, error) {
	e := &OllamaEmbedder{
		url:    ollamaURL,
		model:  model,
		client: &http.Client{Timeout: 30 * time.Second},
	}
	// Probe to detect dimension and verify connectivity.
	vec, err := e.embed(context.Background(), "probe")
	if err != nil {
		return nil, fmt.Errorf("ollama probe failed (is Ollama running? model pulled?): %w", err)
	}
	e.dim = len(vec)
	return e, nil
}

func (e *OllamaEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	return e.embed(ctx, text)
}

func (e *OllamaEmbedder) Dim() int { return e.dim }

// ─── internal ──────────────────────────────────────────────────────────────

type ollamaEmbedReq struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

type ollamaEmbedResp struct {
	Embedding []float32 `json:"embedding"`
}

func (e *OllamaEmbedder) embed(ctx context.Context, text string) ([]float32, error) {
	body, _ := json.Marshal(ollamaEmbedReq{Model: e.model, Prompt: text})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.url+"/api/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama status %d", resp.StatusCode)
	}

	var result ollamaEmbedResp
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("ollama decode: %w", err)
	}
	if len(result.Embedding) == 0 {
		return nil, fmt.Errorf("ollama returned empty embedding — is model '%s' pulled?", e.model)
	}
	return result.Embedding, nil
}
