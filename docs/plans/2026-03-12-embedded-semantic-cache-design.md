# Embedded Semantic Cache Design

## Status
- Validated with the user on `2026-03-12`.

## Problem
- The current service is a semantic cache in name, but its semantic path is lexical scoring based on BM25 + Jaccard in `internal/cache/scorer.go`.
- That approach is lightweight, but it is not a true embedding-based semantic cache and will miss paraphrases that share little token overlap.
- The current gRPC layer in `internal/server/grpc.go` is handwritten rather than generated from `.proto` definitions, which makes long-term protocol evolution harder.

## Goals
- Turn the service into a real embedding-based semantic cache.
- Make gRPC the primary performance path.
- Keep exact-match lookup as the fastest path before semantic search.
- Scope semantic reuse by namespace metadata rather than prompt text alone.
- Support CPU-only embedded model inference inside the service process.
- Target serious single-node usage, roughly `100k` to `1M` entries per node.

## Non-Goals For V1
- Durable persistence across restarts.
- Multi-node replication or shared storage.
- GPU acceleration.
- Rich batch APIs from day one.

## Key Decisions
- **Embedding runtime:** CPU-only embedded model loaded in the service process.
- **Primary protocol:** gRPC is the main API to optimize; HTTP may remain as a thin compatibility layer.
- **Cache identity:** prompt matching happens inside a namespace built from request metadata such as `model`, `tenant`, and `system_prompt_hash`.
- **Persistence:** memory-only for v1.
- **Retrieval order:** exact hash lookup first, vector search second.

## Considered Approaches

### 1. Hybrid exact + vector cache (chosen)
- Keep the current exact normalized-prompt hash path.
- Add embedded-model vectorization and a namespace-aware vector index for misses.
- Preserve the existing strength of the service while making semantic lookup real.

### 2. Flat vector cache only
- Replace BM25/Jaccard with embeddings and do a brute-force scan within each namespace.
- Simpler, but likely too slow near the top end of the target scale.

### 3. Lexical prefilter + vectors
- Use token scoring to shrink the candidate set before vector search.
- Lower CPU cost, but risks missing true semantic neighbors and adds complexity to tuning.

## Recommended Architecture
- Keep a single Go service process.
- Route both protocols through one cache engine, but optimize gRPC first.
- Add an embedding runtime component that loads the CPU model once and exposes `Embed(text) -> []float32`.
- Add a namespace resolver that converts request metadata into a deterministic internal namespace key.
- Extend the entry store to track namespace membership, vector data, TTL, and exact-match keys.
- Introduce a `VectorIndex` abstraction so the retrieval backend can evolve without rewriting the cache engine.

## Main Components
- `Protocol layer`: validates requests, resolves namespaces, and maps cache results to transport responses.
- `Embedding runtime`: owns model loading, inference, and concurrency control.
- `Namespace manager`: builds stable namespace keys from request metadata.
- `Entry store`: keeps payloads, TTL, exact-match metadata, and per-entry stats.
- `Vector index`: searches embeddings within a namespace.
- `Match policy`: applies thresholding, tie-breaking, TTL checks, and hit metadata.

## Data Flow

### Set
- Validate prompt, response, TTL, and namespace fields.
- Normalize the prompt and compute the namespace key.
- Compute the exact-match hash inside that namespace.
- Generate the embedding once.
- Store the payload, metadata, and vector atomically from the caller's point of view.

### Get
- Validate prompt and namespace fields.
- Resolve the namespace key.
- Attempt exact lookup first.
- On exact miss, embed the query prompt.
- Search the namespace-scoped vector index.
- Apply the threshold and return the best hit plus metadata such as `exact_match` and `similarity`.

## Protocol Direction
- Move from handwritten gRPC messages to a proto-first generated API.
- Keep RPCs small in v1: `Get`, `Set`, `Delete`, `Stats`, `Health`.
- Include explicit namespace fields in requests rather than hiding them in an opaque blob.
- Allow request-level threshold override and optional `top_k`, while keeping sane config defaults.

## Index And Retrieval Strategy
- Introduce a `VectorIndex` interface with `Insert`, `Delete`, `Search`, and `Stats`.
- Keep index logic namespace-aware, either through per-namespace indexes or namespace partitioning inside one implementation.
- Start with normalized `[]float32` vectors in memory.
- Use cosine similarity for semantic scoring.
- Design the abstraction so a flat baseline implementation can land first, but an ANN implementation can replace it without breaking the engine.
- Do not keep BM25/Jaccard in the critical path unless benchmarks later prove it adds value.

## Failure Handling
- If the model cannot load at startup, fail fast rather than running with degraded semantics silently.
- If embedding fails during `Set`, return an error and do not create a partial entry.
- If exact lookup misses and embedding fails during `Get`, return an explicit error instead of a false miss.
- Validate namespace metadata in the gRPC layer and reject incomplete requests with `InvalidArgument`.
- Ensure expired entries are removed from both metadata storage and the vector index.

## Operational Guidance
- Add metrics for model load time, embedding latency, exact-hit rate, semantic-hit rate, search latency, namespace counts, and index size.
- Add config for model path, vector dimension, default similarity threshold, max entries, and concurrency limits.
- Add gRPC interceptors for logging, latency metrics, and panic recovery.
- Treat memory and inference concurrency as first-class operational limits.

## Testing Strategy
- Keep the core cache engine testable without gRPC.
- Use a fake embedder for most unit tests.
- Add deterministic tests for namespace isolation, exact-hit precedence, threshold behavior, TTL cleanup, and delete behavior.
- Add gRPC tests for validation, namespace handling, and error mapping.
- Benchmark exact `Get`, semantic `Get`, `Set`, and cleanup paths separately.
- Maintain a small semantic quality corpus to track false positives and false negatives.

## Expected Repo Impact
- `internal/cache/semantic.go` becomes embedding-driven.
- `internal/cache/scorer.go` is likely removed or deprecated.
- `internal/server/grpc.go` should stop hand-defining protobuf messages and instead use generated code.
- New files will be needed for namespace handling, embedder abstraction, vector indexing, and generated protobuf artifacts.

## Risks
- Embedded CPU inference changes the runtime profile of the project and likely removes the current “zero dependencies/no CGO” posture.
- Large in-memory vector sets can pressure RAM, especially near `1M` entries.
- A poor threshold or weak model will create false positives that reduce cache trust.
- Protocol migration from handwritten gRPC messages to generated protobuf code must be carefully tested.

## Follow-Up Decisions During Implementation
- Final embedded model choice, likely a compact sentence-transformer exported to ONNX.
- Initial vector index implementation and when to graduate from flat search to ANN.
- Whether HTTP remains feature-complete or becomes a compatibility/admin surface only.
