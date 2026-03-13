# Embedded Semantic Cache Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Turn the current BM25/Jaccard cache into a namespace-aware embedding-based semantic cache with gRPC as the primary protocol while preserving exact-match fast lookup.

**Architecture:** Replace the lexical semantic path with embedded-model vector retrieval behind a `VectorIndex` abstraction, and migrate the transport to a proto-first gRPC contract. Keep one cache engine shared by gRPC and HTTP, but optimize gRPC and namespace-aware matching first.

**Tech Stack:** Go 1.23.4, gRPC, protobuf codegen, float32 in-memory vectors, existing TTL/LRU metadata store, CPU embedding runtime (prefer ONNX Runtime CPU plus a compact sentence-transformer model), `go test`, `golangci-lint`.

---

## Task 1: Add namespace-aware gRPC tests that define the target contract

**Files:**
- Modify: `internal/server/grpc_test.go`
- Test: `internal/server/grpc_test.go`

**Step 1: Write the failing test**

```go
func TestGRPCServiceRequiresNamespace(t *testing.T) {
	client, cleanup := newBufconnClient(t)
	defer cleanup()

	_, err := client.Get(context.Background(), &embv1.GetRequest{
		Prompt: "What is Go?",
	})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("got code %s, want %s", status.Code(err), codes.InvalidArgument)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/server -run TestGRPCServiceRequiresNamespace -count=1`
Expected: FAIL because the request types do not yet include namespace-aware generated protobuf messages.

**Step 3: Extend the test file with the approved contract expectations**

```go
ns := &embv1.Namespace{
	Model:            "llama3.1-8b",
	TenantId:         "tenant-a",
	SystemPromptHash: "sys-123",
}
```

**Step 4: Run the focused server tests again**

Run: `go test ./internal/server -run 'TestGRPCService(RequiresNamespace|Validation)' -count=1`
Expected: FAIL until the generated protobuf API and server validation are implemented.

**Step 5: Commit**

```bash
git add internal/server/grpc_test.go
git commit -m "test: define namespace-aware grpc contract"
```

## Task 2: Create the proto-first gRPC contract and generation hooks

**Files:**
- Create: `proto/ember/v1/semantic_cache.proto`
- Modify: `Makefile`
- Create: `internal/gen/ember/v1/semantic_cache.pb.go`
- Create: `internal/gen/ember/v1/semantic_cache_grpc.pb.go`

**Step 1: Write the contract file**

```proto
syntax = "proto3";

package ember.v1;

option go_package = "github.com/EricNguyen1206/erion-ember/internal/gen/ember/v1;embv1";

message Namespace {
  string model = 1;
  string tenant_id = 2;
  string system_prompt_hash = 3;
}
```

**Step 2: Add generation commands to the build**

```make
proto:
	protoc \
	  --go_out=. \
	  --go-grpc_out=. \
	  proto/ember/v1/semantic_cache.proto
```

**Step 3: Generate the Go bindings**

Run: `make proto`
Expected: `internal/gen/ember/v1/semantic_cache.pb.go` and `internal/gen/ember/v1/semantic_cache_grpc.pb.go` are created or refreshed.

**Step 4: Verify the package builds**

Run: `go test ./internal/server -run TestGRPCServiceRequiresNamespace -count=1`
Expected: FAIL in handler wiring or validation, not in missing generated types.

**Step 5: Commit**

```bash
git add Makefile proto/ember/v1/semantic_cache.proto internal/gen/ember/v1/semantic_cache.pb.go internal/gen/ember/v1/semantic_cache_grpc.pb.go
git commit -m "build: add generated protobuf contract"
```

## Task 3: Replace the handwritten gRPC transport with generated protobuf types

**Files:**
- Modify: `internal/server/grpc.go`
- Modify: `internal/server/grpc_test.go`

**Step 1: Write the failing handler validation path**

```go
if req == nil || !hasText(req.Prompt) || req.Namespace == nil || !hasText(req.Namespace.Model) {
	return nil, status.Error(codes.InvalidArgument, "namespace is required")
}
```

**Step 2: Run the focused server tests**

Run: `go test ./internal/server -run 'TestGRPCService(RequiresNamespace|CRUDFlow|Validation)' -count=1`
Expected: FAIL until the server registers and uses generated request and response types.

**Step 3: Implement the transport migration**

```go
type semanticCacheService struct {
	embv1.UnimplementedSemanticCacheServiceServer
	cache *cache.SemanticCache
}
```

**Step 4: Re-run the focused server tests**

Run: `go test ./internal/server -run 'TestGRPCService(RequiresNamespace|CRUDFlow|Validation)' -count=1`
Expected: PASS for transport validation once handler wiring is complete.

**Step 5: Commit**

```bash
git add internal/server/grpc.go internal/server/grpc_test.go
git commit -m "refactor: switch grpc server to generated protobuf types"
```

## Task 4: Add namespace resolution and embedder abstractions in the cache layer

**Files:**
- Create: `internal/cache/namespace.go`
- Create: `internal/cache/embedder.go`
- Create: `internal/cache/namespace_test.go`
- Create: `internal/cache/embedder_test.go`

**Step 1: Write the failing namespace test**

```go
func TestNamespaceKeyStable(t *testing.T) {
	key := cache.NamespaceKey(cache.Namespace{
		Model:            "llama3.1-8b",
		TenantID:         "tenant-a",
		SystemPromptHash: "sys-123",
	})
	if key == "" {
		t.Fatal("expected non-empty namespace key")
	}
}
```

**Step 2: Run the focused cache test**

Run: `go test ./internal/cache -run TestNamespaceKeyStable -count=1`
Expected: FAIL because namespace support does not exist yet.

**Step 3: Add the minimal abstractions**

```go
type Namespace struct {
	Model            string
	TenantID         string
	SystemPromptHash string
}

type Embedder interface {
	Embed(context.Context, string) ([]float32, error)
	Dimension() int
}
```

**Step 4: Re-run the focused tests**

Run: `go test ./internal/cache -run 'Test(NamespaceKeyStable|FakeEmbedderDimension)' -count=1`
Expected: PASS.

**Step 5: Commit**

```bash
git add internal/cache/namespace.go internal/cache/embedder.go internal/cache/namespace_test.go internal/cache/embedder_test.go
git commit -m "feat: add namespace and embedder abstractions"
```

## Task 5: Extend entry storage for namespace-aware exact lookup and vector-bearing entries

**Files:**
- Modify: `internal/cache/metadata.go`
- Modify: `internal/cache/metadata_test.go`

**Step 1: Write the failing storage test**

```go
func TestMetadataStoreSeparatesNamespaces(t *testing.T) {
	store := cache.NewMetadataStore(10)
	store.Set(cache.EntryKey("tenant-a:model-a", 1), &cache.Entry{NamespaceKey: "tenant-a:model-a"}, 0)
	store.Set(cache.EntryKey("tenant-b:model-a", 1), &cache.Entry{NamespaceKey: "tenant-b:model-a"}, 0)

	if store.Len() != 2 {
		t.Fatalf("got %d entries, want 2", store.Len())
	}
}
```

**Step 2: Run the focused metadata test**

Run: `go test ./internal/cache -run TestMetadataStoreSeparatesNamespaces -count=1`
Expected: FAIL because keys are still prompt-hash only.

**Step 3: Implement composite exact keys and vector-bearing entries**

```go
type Entry struct {
	ID           string
	NamespaceKey string
	PromptHash   uint64
	Vector       []float32
	Response     []byte
	// existing TTL and accounting fields stay here
}
```

**Step 4: Re-run metadata tests**

Run: `go test ./internal/cache -run 'TestMetadataStore(SeparatesNamespaces|TTL|LRU)' -count=1`
Expected: PASS.

**Step 5: Commit**

```bash
git add internal/cache/metadata.go internal/cache/metadata_test.go
git commit -m "feat: add namespace-aware entry storage"
```

## Task 6: Add a pluggable vector index with a flat-search baseline

**Files:**
- Create: `internal/cache/vector_index.go`
- Create: `internal/cache/flat_index.go`
- Create: `internal/cache/flat_index_test.go`

**Step 1: Write the failing vector search test**

```go
func TestFlatIndexSearchReturnsNearestNeighbor(t *testing.T) {
	idx := cache.NewFlatIndex()
	idx.Insert("ns-a", "1", []float32{1, 0})
	idx.Insert("ns-a", "2", []float32{0, 1})

	got := idx.Search("ns-a", []float32{0.9, 0.1}, 1)
	if len(got) != 1 || got[0].EntryID != "1" {
		t.Fatalf("got %#v, want entry 1", got)
	}
}
```

**Step 2: Run the focused index test**

Run: `go test ./internal/cache -run TestFlatIndexSearchReturnsNearestNeighbor -count=1`
Expected: FAIL because no vector index exists yet.

**Step 3: Implement the index interface and flat baseline**

```go
type VectorIndex interface {
	Insert(namespaceKey, entryID string, vector []float32)
	Delete(namespaceKey, entryID string)
	Search(namespaceKey string, query []float32, topK int) []SearchResult
	Stats() IndexStats
}
```

**Step 4: Re-run the focused index tests**

Run: `go test ./internal/cache -run 'TestFlatIndex(SearchReturnsNearestNeighbor|Delete)' -count=1`
Expected: PASS.

**Step 5: Commit**

```bash
git add internal/cache/vector_index.go internal/cache/flat_index.go internal/cache/flat_index_test.go
git commit -m "feat: add pluggable vector index baseline"
```

## Task 7: Replace BM25 retrieval with exact-hash plus vector search in the cache engine

**Files:**
- Modify: `internal/cache/semantic.go`
- Modify: `internal/cache/semantic_test.go`
- Modify: `internal/cache/scorer.go`

**Contract note:** `Get` should return `(*GetResult, error)`. A semantic or exact hit returns a populated result and `nil` error. A cache miss returns `nil, nil`. An embedding or index failure during `Get` returns `nil, err` so transports can map infrastructure failures explicitly instead of collapsing them into misses.

**Step 1: Write the failing semantic behavior test**

```go
func TestSemanticCacheDoesNotCrossNamespaces(t *testing.T) {
	ctx := context.Background()
	sc := newVectorBackedCache(fakeEmbedder())

	_, _ = sc.Set(ctx, cache.SetInput{
		Namespace: cache.Namespace{Model: "m1", TenantID: "a", SystemPromptHash: "sys"},
		Prompt:    "what is go",
		Response:  "go is a language",
	})

	result, err := sc.Get(ctx, cache.GetInput{
		Namespace: cache.Namespace{Model: "m1", TenantID: "b", SystemPromptHash: "sys"},
		Prompt:    "tell me about golang",
	})
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if result != nil {
		t.Fatal("expected miss across namespaces")
	}
}
```

**Step 2: Run the focused semantic tests**

Run: `go test ./internal/cache -run 'TestSemanticCache(DoesNotCrossNamespaces|ExactHitWins|Threshold)' -count=1`
Expected: FAIL until the cache engine uses namespaces, embeddings, and vector search.

**Step 3: Implement the new engine path**

```go
// Get order:
// 1. exact lookup by namespace + normalized prompt hash
// 2. embed query
// 3. search vector index within namespace
// 4. apply threshold and return best hit as (*GetResult, nil)
// 5. return (nil, nil) on miss and (nil, err) on embedding/index failure
```

**Step 4: Re-run the focused semantic tests**

Run: `go test ./internal/cache -run 'TestSemanticCache(DoesNotCrossNamespaces|ExactHitWins|Threshold|DeleteRemovesVector|TTLSkipsExpired)' -count=1`
Expected: PASS.

**Step 5: Commit**

```bash
git add internal/cache/semantic.go internal/cache/semantic_test.go internal/cache/scorer.go
git commit -m "feat: switch cache engine to embedding retrieval"
```

## Task 8: Wire the embedded CPU model runtime and startup configuration

**Files:**
- Create: `internal/cache/onnx_embedder.go`
- Create: `internal/cache/onnx_embedder_test.go`
- Modify: `cmd/server/main.go`
- Modify: `go.mod`

**Step 1: Write the failing startup/config test**

```go
func TestLoadConfigRequiresModelPathWhenSemanticEnabled(t *testing.T) {
	// set env, call loadConfig, assert model path is captured
}
```

**Step 2: Run the focused startup test**

Run: `go test ./cmd/server -run TestLoadConfigRequiresModelPathWhenSemanticEnabled -count=1`
Expected: FAIL because model config and runtime wiring do not exist yet.

**Step 3: Implement the runtime wiring**

```go
type ModelConfig struct {
	Path      string
	Dimension int
	Workers   int
}
```

**Step 4: Re-run the focused runtime tests**

Run: `go test ./cmd/server ./internal/cache -run 'Test(LoadConfigRequiresModelPathWhenSemanticEnabled|ONNXEmbedderDimension)' -count=1`
Expected: PASS for config and runtime unit coverage; integration tests may be skipped when model assets are absent.

**Step 5: Commit**

```bash
git add cmd/server/main.go internal/cache/onnx_embedder.go internal/cache/onnx_embedder_test.go go.mod go.sum
git commit -m "feat: wire embedded cpu embedding runtime"
```

## Task 9: Update HTTP compatibility, stats, and documentation

**Files:**
- Modify: `internal/server/http.go`
- Modify: `internal/server/http_test.go`
- Modify: `README.md`
- Modify: `docs/API_REFERENCE.md`
- Modify: `AGENTS.md`

**Step 1: Write the failing HTTP compatibility test**

```go
func TestHTTPGetRequiresNamespaceFields(t *testing.T) {
	// post a get request without namespace metadata and expect 400
}
```

**Step 2: Run the focused HTTP test**

Run: `go test ./internal/server -run TestHTTPGetRequiresNamespaceFields -count=1`
Expected: FAIL because HTTP payloads do not yet carry namespace metadata.

**Step 3: Implement the thin compatibility layer and docs updates**

```go
type namespaceJSON struct {
	Model            string `json:"model"`
	TenantID         string `json:"tenant_id"`
	SystemPromptHash string `json:"system_prompt_hash"`
}
```

**Step 4: Re-run focused transport tests**

Run: `go test ./internal/server -run 'TestHTTP(GetRequiresNamespaceFields|CRUDFlow)' -count=1`
Expected: PASS.

**Step 5: Commit**

```bash
git add internal/server/http.go internal/server/http_test.go README.md docs/API_REFERENCE.md AGENTS.md
git commit -m "docs: align http compatibility and semantic cache docs"
```

## Task 10: Add benchmarks, full-suite validation, and final cleanup

**Files:**
- Modify: `internal/cache/semantic_test.go`
- Modify: `internal/server/grpc_test.go`
- Modify: `Makefile`

**Step 1: Write the benchmark and regression coverage**

```go
func BenchmarkSemanticGetVectorHit(b *testing.B) {
	// seed cache, then benchmark namespace-scoped semantic gets
}
```

**Step 2: Run focused benchmarks and tests**

Run: `go test ./internal/cache -run '^$' -bench 'BenchmarkSemanticGet(VectorHit|ExactHit)'`
Expected: PASS and emit baseline benchmark numbers.

**Step 3: Run repository validation**

Run: `gofmt -w cmd/server/main.go internal/cache/*.go internal/server/*.go`
Run: `go test ./...`
Run: `golangci-lint run ./...`
Known baseline failures before this plan starts:
- `go test -race ./...` crashes with `checkptr: misaligned pointer conversion` in the handwritten gRPC layer.
- `golangci-lint run ./...` reports staticcheck deprecations in `internal/server/grpc.go` and `internal/server/grpc_test.go`.
Expected: new work should not introduce additional failures; once the gRPC transport is migrated, all commands should pass. If `golangci-lint` is unavailable, record that explicitly.

**Step 4: Update final docs or defaults discovered during validation**

```md
- confirm default ports
- confirm model env vars
- confirm grpc is the primary protocol in README
```

**Step 5: Commit**

```bash
git add Makefile cmd/server/main.go internal/cache internal/server README.md docs/API_REFERENCE.md AGENTS.md
git commit -m "test: add semantic cache benchmarks and final validation"
```

## Notes For The Implementer
- Keep the exact lookup path cheap and allocation-aware.
- Do not silently degrade to lexical matching when embedding inference fails.
- Prefer a fake embedder in tests and only load a real model in explicit integration tests.
- Keep HTTP thin and avoid introducing business logic there.
- If the ANN backend is not ready, land the `VectorIndex` abstraction and flat baseline first, then benchmark before swapping implementations.

## Validation Checklist
- `go test ./internal/cache -count=1`
- `go test ./internal/server -count=1`
- `go test ./...`
- `go test ./internal/cache -run '^$' -bench .`
- `golangci-lint run ./...`
