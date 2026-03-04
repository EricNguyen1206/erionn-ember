# Erion Ember v3 — Go Rewrite Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Rewrite erion-ember completely in Go as a standalone gRPC + REST semantic cache service.

**Architecture:** Single binary with ONNX Worker Pool. Fast-path (xxhash LRU hit, ~0.1ms) never blocked by slow-path (ONNX embed → HNSW search). gRPC is primary protocol; grpc-gateway exposes REST/JSON on a second port.

**Tech Stack:** Go 1.22+, `google.golang.org/grpc`, `github.com/grpc-ecosystem/grpc-gateway/v2`, `github.com/knights-analytics/hugot` (ONNX/HuggingFace), `github.com/unum-cloud/usearch` (HNSW), `github.com/cespare/xxhash/v2`, `github.com/pierrec/lz4/v4`

---

## Task 0: Cleanup — Delete TypeScript/Node.js files

**Files to delete:**
- `src/` (entire directory)
- `tests/` (entire directory)
- `node_modules/` (entire directory — do NOT commit this, just remove)
- `test-qdrant.ts`
- `bun.lock`
- `package.json`
- `tsconfig.json`
- `.npmrc`
- `erion-ember-mcp` (compiled binary artifact if present)

**Step 1: Remove Node.js/TypeScript files**
```bash
rm -rf src/ tests/ node_modules/ test-qdrant.ts bun.lock package.json tsconfig.json .npmrc erion-ember-mcp
```

**Step 2: Update .gitignore for Go**

Replace contents of `.gitignore` with Go-appropriate entries:
```gitignore
# Go
bin/
dist/
*.exe
*.test
*.out
coverage.out

# ONNX model cache
models/

# Environment
.env

# OS
.DS_Store

# IDE
.idea/
.vscode/
*.swp
```

**Step 3: Commit cleanup**
```bash
git add -A
git commit -m "chore: remove TypeScript/Node.js files, begin Go rewrite"
```

---

## Task 1: Go module + project skeleton

**Files:**
- Create: `go.mod`
- Create: `cmd/server/main.go` (stub)
- Create: `proto/ember/v1/cache.proto`

**Step 1: Initialize Go module**
```bash
go mod init github.com/EricNguyen1206/erion-ember
```

**Step 2: Create proto file**

Create `proto/ember/v1/cache.proto`:
```proto
syntax = "proto3";
package ember.v1;
option go_package = "github.com/EricNguyen1206/erion-ember/gen/ember/v1";

import "google/api/annotations.proto";
import "google/protobuf/empty.proto";

service CacheService {
  rpc Get    (GetRequest)    returns (GetResponse) {
    option (google.api.http) = { post: "/v1/cache/get",    body: "*" };
  }
  rpc Set    (SetRequest)    returns (SetResponse) {
    option (google.api.http) = { post: "/v1/cache/set",    body: "*" };
  }
  rpc Delete (DeleteRequest) returns (DeleteResponse) {
    option (google.api.http) = { post: "/v1/cache/delete", body: "*" };
  }
  rpc Stats  (google.protobuf.Empty) returns (StatsResponse) {
    option (google.api.http) = { get:  "/v1/stats" };
  }
}

message GetRequest {
  string prompt = 1;
  optional float similarity_threshold = 2;
}
message GetResponse {
  bool   hit         = 1;
  string response    = 2;
  float  similarity  = 3;
  bool   exact_match = 4;
}
message SetRequest {
  string prompt   = 1;
  string response = 2;
  optional int32  ttl = 3;
}
message SetResponse { string id = 1; }
message DeleteRequest { string prompt = 1; }
message DeleteResponse { bool deleted = 1; }
message StatsResponse {
  int64  total_entries     = 1;
  int64  cache_hits        = 2;
  int64  cache_misses      = 3;
  float  hit_rate          = 4;
  string compression_ratio = 5;
}
```

**Step 3: Install protoc tools and generate code**
```bash
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway@latest

mkdir -p gen/ember/v1

protoc -I proto \
  -I $(go env GOPATH)/pkg/mod/github.com/grpc-ecosystem/grpc-gateway/v2@*/third_party/googleapis \
  --go_out=gen --go_opt=paths=source_relative \
  --go-grpc_out=gen --go-grpc_opt=paths=source_relative \
  --grpc-gateway_out=gen --grpc-gateway_opt=paths=source_relative \
  proto/ember/v1/cache.proto
```

**Step 4: Create stub main.go**

Create `cmd/server/main.go`:
```go
package main

import "fmt"

func main() {
    fmt.Println("erion-ember v3")
}
```

**Step 5: Verify build**
```bash
go build ./...
```
Expected: no errors.

**Step 6: Add core dependencies**
```bash
go get google.golang.org/grpc
go get github.com/grpc-ecosystem/grpc-gateway/v2
go get github.com/cespare/xxhash/v2
go get github.com/pierrec/lz4/v4
go get github.com/knights-analytics/hugot
```

**Step 7: Commit**
```bash
git add -A
git commit -m "feat: initialize Go module, proto definition, gen code"
```

---

## Task 2: Normalizer + xxhash (with tests)

**Files:**
- Create: `internal/cache/normalizer.go`
- Create: `internal/cache/normalizer_test.go`

**Step 1: Write failing test**

Create `internal/cache/normalizer_test.go`:
```go
package cache_test

import (
    "testing"
    "github.com/EricNguyen1206/erion-ember/internal/cache"
)

func TestNormalize(t *testing.T) {
    n := cache.NewNormalizer()
    cases := []struct{ in, want string }{
        {"Hello World",    "hello world"},
        {"  spaces  ",     "spaces"},
        {"UPPER CASE",     "upper case"},
        {"multi   space",  "multi space"},
    }
    for _, c := range cases {
        got := n.Normalize(c.in)
        if got != c.want {
            t.Errorf("Normalize(%q) = %q, want %q", c.in, got, c.want)
        }
    }
}

func TestHashConsistency(t *testing.T) {
    n := cache.NewNormalizer()
    h1 := n.Hash("hello world")
    h2 := n.Hash("hello world")
    if h1 != h2 {
        t.Error("Hash must be deterministic")
    }
}

func TestHashDifferent(t *testing.T) {
    n := cache.NewNormalizer()
    if n.Hash("foo") == n.Hash("bar") {
        t.Error("Different inputs should produce different hashes")
    }
}
```

**Step 2: Run — expect FAIL**
```bash
go test ./internal/cache/... -run TestNormalize -v
```
Expected: `cannot find package` or `undefined: cache.NewNormalizer`

**Step 3: Implement**

Create `internal/cache/normalizer.go`:
```go
package cache

import (
    "strings"
    "unicode"

    "github.com/cespare/xxhash/v2"
)

type Normalizer struct{}

func NewNormalizer() *Normalizer { return &Normalizer{} }

// Normalize lowercases, trims, and collapses internal whitespace.
func (n *Normalizer) Normalize(text string) string {
    text = strings.ToLower(text)
    text = strings.TrimSpace(text)
    // collapse multiple spaces
    var b strings.Builder
    prevSpace := false
    for _, r := range text {
        if unicode.IsSpace(r) {
            if !prevSpace {
                b.WriteRune(' ')
            }
            prevSpace = true
        } else {
            b.WriteRune(r)
            prevSpace = false
        }
    }
    return b.String()
}

// Hash returns a 64-bit xxhash of normalized text.
func (n *Normalizer) Hash(normalized string) uint64 {
    return xxhash.Sum64String(normalized)
}
```

**Step 4: Run — expect PASS**
```bash
go test ./internal/cache/... -run "TestNormalize|TestHash" -v
```
Expected: `PASS`

**Step 5: Commit**
```bash
git add internal/cache/normalizer.go internal/cache/normalizer_test.go
git commit -m "feat: add Normalizer with xxhash"
```

---

## Task 3: LZ4 Compressor (with tests)

**Files:**
- Create: `internal/cache/compressor.go`
- Create: `internal/cache/compressor_test.go`

**Step 1: Write failing test**

Create `internal/cache/compressor_test.go`:
```go
package cache_test

import (
    "testing"
    "github.com/EricNguyen1206/erion-ember/internal/cache"
)

func TestCompressRoundtrip(t *testing.T) {
    c := cache.NewCompressor()
    original := "The quick brown fox jumps over the lazy dog."
    compressed := c.Compress(original)
    got, err := c.Decompress(compressed, len(original))
    if err != nil {
        t.Fatal(err)
    }
    if got != original {
        t.Errorf("roundtrip failed: got %q, want %q", got, original)
    }
}

func TestCompressReducesSize(t *testing.T) {
    c := cache.NewCompressor()
    // repeated text compresses well
    text := strings.Repeat("hello world ", 100)
    compressed := c.Compress(text)
    if len(compressed) >= len(text) {
        t.Errorf("expected compression, got %d >= %d", len(compressed), len(text))
    }
}
```

**Step 2: Run — expect FAIL**
```bash
go test ./internal/cache/... -run TestCompress -v
```

**Step 3: Implement**

Create `internal/cache/compressor.go`:
```go
package cache

import (
    "fmt"
    "github.com/pierrec/lz4/v4"
)

type Compressor struct{}

func NewCompressor() *Compressor { return &Compressor{} }

func (c *Compressor) Compress(text string) []byte {
    src := []byte(text)
    dst := make([]byte, lz4.CompressBlockBound(len(src)))
    n, err := lz4.CompressBlock(src, dst, nil)
    if err != nil || n == 0 {
        return src // fallback: store uncompressed
    }
    return dst[:n]
}

func (c *Compressor) Decompress(data []byte, originalSize int) (string, error) {
    dst := make([]byte, originalSize)
    n, err := lz4.UncompressBlock(data, dst)
    if err != nil {
        return "", fmt.Errorf("lz4 decompress: %w", err)
    }
    return string(dst[:n]), nil
}
```

**Step 4: Run — expect PASS**
```bash
go test ./internal/cache/... -run TestCompress -v
```

**Step 5: Commit**
```bash
git add internal/cache/compressor.go internal/cache/compressor_test.go
git commit -m "feat: add LZ4 Compressor"
```

---

## Task 4: LRU Metadata Store (with tests)

**Files:**
- Create: `internal/cache/metadata.go`
- Create: `internal/cache/metadata_test.go`

**Concept:** Thread-safe LRU map keyed by `uint64` (xxhash). Each entry has `vectorId int`, `compressedPrompt []byte`, `compressedResponse []byte`, `originalResponseSize int`, `createdAt time.Time`, `expiresAt *time.Time`.

**Step 1: Write failing test**

Create `internal/cache/metadata_test.go`:
```go
package cache_test

import (
    "testing"
    "time"
    "github.com/EricNguyen1206/erion-ember/internal/cache"
)

func TestMetadataSetGet(t *testing.T) {
    store := cache.NewMetadataStore(100)
    entry := &cache.Entry{
        VectorId:             1,
        CompressedResponse:   []byte("resp"),
        OriginalResponseSize: 10,
    }
    store.Set(42, entry, 0) // 0 = no TTL
    got, ok := store.Get(42)
    if !ok {
        t.Fatal("expected entry not found")
    }
    if got.VectorId != 1 {
        t.Errorf("got VectorId %d, want 1", got.VectorId)
    }
}

func TestMetadataTTLExpiry(t *testing.T) {
    store := cache.NewMetadataStore(100)
    store.Set(99, &cache.Entry{}, 1) // 1 second TTL
    time.Sleep(1100 * time.Millisecond)
    _, ok := store.Get(99)
    if ok {
        t.Error("entry should have expired")
    }
}

func TestMetadataLRUEviction(t *testing.T) {
    store := cache.NewMetadataStore(2) // max 2 entries
    store.Set(1, &cache.Entry{VectorId: 1}, 0)
    store.Set(2, &cache.Entry{VectorId: 2}, 0)
    store.Set(3, &cache.Entry{VectorId: 3}, 0) // evicts key 1
    _, ok := store.Get(1)
    if ok {
        t.Error("key 1 should have been evicted")
    }
}
```

**Step 2: Run — expect FAIL**
```bash
go test ./internal/cache/... -run TestMetadata -v
```

**Step 3: Implement** `internal/cache/metadata.go` — thread-safe LRU using a doubly-linked list + map, with TTL check on `Get`. See `container/list` from stdlib.

**Step 4: Run — expect PASS**
```bash
go test ./internal/cache/... -run TestMetadata -v
```

**Step 5: Commit**
```bash
git add internal/cache/metadata.go internal/cache/metadata_test.go
git commit -m "feat: add LRU MetadataStore with TTL"
```

---

## Task 5: ONNX Embedding Worker Pool

**Files:**
- Create: `internal/embedding/onnx.go`
- Create: `internal/embedding/pool.go`
- Create: `internal/embedding/pool_test.go`

**Step 1: Download ONNX model**

`hugot` will auto-download the model on first run to a local `models/` directory. Add `models/` to `.gitignore`.

**Step 2: Write failing pool test**

Create `internal/embedding/pool_test.go`:
```go
package embedding_test

import (
    "context"
    "testing"
    "github.com/EricNguyen1206/erion-ember/internal/embedding"
)

// This test requires the ONNX model to be present.
// Run: ONNX_MODEL_PATH=models/all-MiniLM-L6-v2.onnx go test ./internal/embedding/...
func TestPoolEmbed(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping ONNX test in short mode")
    }
    pool, err := embedding.NewPool(1, "models/")
    if err != nil {
        t.Fatalf("NewPool: %v", err)
    }
    defer pool.Close()

    vec, err := pool.Embed(context.Background(), "hello world")
    if err != nil {
        t.Fatalf("Embed: %v", err)
    }
    if len(vec) != 384 {
        t.Errorf("expected 384-dim, got %d", len(vec))
    }
}
```

**Step 3: Run — expect FAIL**
```bash
go test -short ./internal/embedding/... -v
```
Expected: `SKIP` (short mode)

**Step 4: Implement**

`internal/embedding/onnx.go` — wraps `hugot` pipeline to load `all-MiniLM-L6-v2`.

`internal/embedding/pool.go`:
```go
package embedding

import (
    "context"
    "runtime"
)

type EmbedJob struct {
    Prompt   string
    RespChan chan<- EmbedResult
}

type EmbedResult struct {
    Vec []float32
    Err error
}

type Pool struct {
    jobChan chan EmbedJob
    done    chan struct{}
}

func NewPool(workers int, modelDir string) (*Pool, error) {
    if workers <= 0 {
        workers = runtime.NumCPU()
    }
    p := &Pool{
        jobChan: make(chan EmbedJob, 512),
        done:    make(chan struct{}),
    }
    for i := 0; i < workers; i++ {
        runner, err := newONNXRunner(modelDir)
        if err != nil {
            return nil, err
        }
        go p.worker(runner)
    }
    return p, nil
}

func (p *Pool) Embed(ctx context.Context, prompt string) ([]float32, error) {
    ch := make(chan EmbedResult, 1)
    select {
    case p.jobChan <- EmbedJob{Prompt: prompt, RespChan: ch}:
    case <-ctx.Done():
        return nil, ctx.Err()
    }
    select {
    case result := <-ch:
        return result.Vec, result.Err
    case <-ctx.Done():
        return nil, ctx.Err()
    }
}

func (p *Pool) Close() { close(p.done) }

func (p *Pool) worker(runner *onnxRunner) {
    for {
        select {
        case job := <-p.jobChan:
            vec, err := runner.embed(job.Prompt)
            job.RespChan <- EmbedResult{Vec: vec, Err: err}
        case <-p.done:
            return
        }
    }
}
```

**Step 5: Commit**
```bash
git add internal/embedding/
git commit -m "feat: add ONNX embedding worker pool"
```

---

## Task 6: HNSW Index Wrapper

**Files:**
- Create: `internal/index/hnsw.go`
- Create: `internal/index/hnsw_test.go`

**Step 1: Add usearch dependency**
```bash
go get github.com/unum-cloud/usearch/golang
```

**Step 2: Write failing test**

`internal/index/hnsw_test.go`:
```go
package index_test

import (
    "testing"
    "github.com/EricNguyen1206/erion-ember/internal/index"
)

func TestHNSWAddAndSearch(t *testing.T) {
    h, err := index.NewHNSW(4, 1000) // dim=4, maxElements=1000
    if err != nil {
        t.Fatal(err)
    }
    vec := []float32{1, 0, 0, 0}
    id, err := h.Add(vec)
    if err != nil {
        t.Fatal(err)
    }

    results, err := h.Search(vec, 1)
    if err != nil {
        t.Fatal(err)
    }
    if len(results) == 0 {
        t.Fatal("expected at least 1 result")
    }
    if results[0].ID != id {
        t.Errorf("expected id %d, got %d", id, results[0].ID)
    }
    if results[0].Distance > 0.01 {
        t.Errorf("expected near-zero distance, got %f", results[0].Distance)
    }
}
```

**Step 3: Run — expect FAIL**
```bash
go test ./internal/index/... -v
```

**Step 4: Implement** `internal/index/hnsw.go` — thin wrapper around usearch's Go API for `Add`, `Search(vec, k) []SearchResult`, and `Len()`.

**Step 5: Run — expect PASS**
```bash
go test ./internal/index/... -v
```

**Step 6: Commit**
```bash
git add internal/index/
git commit -m "feat: add HNSW index wrapper"
```

---

## Task 7: SemanticCache Orchestrator

**Files:**
- Create: `internal/cache/semantic.go`
- Create: `internal/cache/semantic_test.go`

**Concept:** Wires Normalizer + MetadataStore + Compressor + EmbedPool + HNSW. Implements `Get(ctx, prompt, threshold) (CacheHit, bool)` and `Set(ctx, prompt, response, ttl)`.

**Step 1: Write failing integration test** (uses a mock EmbedPool that returns a fixed vector):

```go
package cache_test

func TestSemanticCacheHitMiss(t *testing.T) {
    // Use mock embedder that returns fixed 4-dim vector
    sc := cache.NewSemanticCacheWithMock(4, 100, 0.85)
    ctx := context.Background()

    err := sc.Set(ctx, "What is Go?", "Go is a language.", 0)
    if err != nil {
        t.Fatal(err)
    }

    hit, ok := sc.Get(ctx, "What is Go?", 0.85)
    if !ok {
        t.Fatal("expected cache hit")
    }
    if !hit.ExactMatch {
        t.Error("expected exact match on identical prompt")
    }

    _, ok = sc.Get(ctx, "totally different query xyz123", 0.85)
    if ok {
        t.Error("expected cache miss")
    }
}
```

**Step 2: Run — expect FAIL**
**Step 3: Implement** `internal/cache/semantic.go`
**Step 4: Run — expect PASS**
```bash
go test ./internal/cache/... -v
```
**Step 5: Commit**
```bash
git add internal/cache/semantic.go internal/cache/semantic_test.go
git commit -m "feat: add SemanticCache orchestrator"
```

---

## Task 8: gRPC Server + grpc-gateway

**Files:**
- Create: `internal/server/grpc.go`
- Create: `internal/server/gateway.go`
- Modify: `cmd/server/main.go`

**Step 1: Implement gRPC handlers** in `internal/server/grpc.go` — implement `CacheServiceServer` interface (generated), wire to `SemanticCache`.

**Step 2: Implement grpc-gateway** in `internal/server/gateway.go`:
```go
func RunGateway(ctx context.Context, grpcAddr, httpAddr string) error {
    mux := runtime.NewServeMux()
    opts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}
    err := gen.RegisterCacheServiceHandlerFromEndpoint(ctx, mux, grpcAddr, opts)
    if err != nil {
        return err
    }
    return http.ListenAndServe(httpAddr, mux)
}
```

**Step 3: Wire main.go**
```go
func main() {
    pool, _ := embedding.NewPool(0, "models/")
    sc := cache.NewSemanticCache(384, 100000, 0.85, pool)
    
    go server.RunGRPC(sc, ":50051")
    go server.RunGateway(ctx, "localhost:50051", ":8080")
    
    // wait for SIGINT/SIGTERM
}
```

**Step 4: Build**
```bash
go build ./cmd/server/...
```
Expected: success.

**Step 5: Smoke test (manual)**
```bash
# Terminal 1
./bin/erion-ember

# Terminal 2 — REST
curl -s -XPOST http://localhost:8080/v1/cache/set \
  -d '{"prompt":"hello","response":"world"}' | jq

curl -s -XPOST http://localhost:8080/v1/cache/get \
  -d '{"prompt":"hello"}' | jq
# Expected: {"hit":true,"response":"world","similarity":1,"exactMatch":true}

curl -s http://localhost:8080/v1/stats | jq
```

**Step 6: Commit**
```bash
git add internal/server/ cmd/server/main.go
git commit -m "feat: add gRPC server + grpc-gateway HTTP proxy"
```

---

## Task 9: Dockerfile + docker-compose

**Files:**
- Modify: `Dockerfile`
- Modify: `docker-compose.yml`

**Step 1: Write Dockerfile**
```dockerfile
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /bin/erion-ember ./cmd/server

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
COPY --from=builder /bin/erion-ember /bin/erion-ember
EXPOSE 50051 8080
ENTRYPOINT ["/bin/erion-ember"]
```

**Step 2: Write docker-compose.yml**
```yaml
version: "3.9"
services:
  ember:
    build: .
    ports:
      - "50051:50051"
      - "8080:8080"
    environment:
      - EMBED_WORKERS=4
      - CACHE_MAX_ELEMENTS=100000
      - CACHE_DEFAULT_TTL=3600
      - CACHE_SIMILARITY_THRESHOLD=0.85
    volumes:
      - ./models:/app/models
```

**Step 3: Build and run**
```bash
docker compose up --build
```
Expected: server starts, `curl http://localhost:8080/v1/stats` returns JSON.

**Step 4: Commit**
```bash
git add Dockerfile docker-compose.yml
git commit -m "feat: Go Dockerfile + docker-compose"
```

---

## Task 10: Update README + final cleanup

- Rewrite `README.md` to document gRPC + REST API, Docker usage, env vars
- Update `CHANGELOG.md` with v3.0.0 entry
- Run full test suite: `go test ./...`
- Final commit: `git commit -m "docs: update README for v3 Go rewrite"`

---

## Environment Variables Reference

| Variable | Default | Description |
|---|---|---|
| `GRPC_PORT` | `50051` | gRPC server port |
| `HTTP_PORT` | `8080` | REST gateway port |
| `EMBED_WORKERS` | `runtime.NumCPU()` | ONNX worker goroutines |
| `EMBED_QUEUE_SIZE` | `512` | Job channel buffer depth |
| `MODEL_DIR` | `models/` | Directory for ONNX model files |
| `CACHE_MAX_ELEMENTS` | `100000` | HNSW max capacity |
| `CACHE_DEFAULT_TTL` | `3600` | Default TTL in seconds (0 = no TTL) |
| `CACHE_SIMILARITY_THRESHOLD` | `0.85` | Default cosine similarity threshold |
