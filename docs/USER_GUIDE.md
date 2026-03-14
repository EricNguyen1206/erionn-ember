# Erion Ember User Guide

![Erion Ember Logo](../assets/logo-horizontal.svg)

Welcome to Erion Ember. Release 1 is a self-hosted, single-node, memory-only semantic cache for chat backends.

## What is Erion Ember?

Erion Ember is a fast cache for prompt/response pairs. It tries an exact prompt match first, then falls back to lexical similarity scoring with BM25 + Jaccard when prompts share enough overlapping terms.

### Why Use It?
- **Save Costs**: Reuse LLM responses for similar queries.
- **Reduce Latency**: Cached hits return in milliseconds.
- **Simple Deployment**: Run one in-memory node with no external model or vector service.

## Release 1 Scope

- Self-hosted, single-node, memory-only deployment.
- gRPC is the preferred integration path for application backends.
- HTTP remains supported for simple integrations, testing, and debugging.
- No persistence, clustering, embeddings, or bundled SDKs in release 1.
- Raw examples live in `examples/go/raw-grpc-chat/main.go`, `examples/go/raw-http-chat/main.go`, `examples/node/raw-grpc-chat/index.js`, and `examples/node/raw-http-chat/index.js`. The Node gRPC example keeps things raw with `@grpc/grpc-js` plus `@grpc/proto-loader` against `proto/ember/v1/semantic_cache.proto`, and the Node examples assume `Node >= 18`.
- The guided release demo lives in `examples/showcase/README.md` and `examples/showcase/scripts/run-demo.sh`.

## Getting Started

### 1. Installation
Build the binary directly from source:
```bash
make build
```

### 2. Start the Service
Run the binary to start the server on port 8080:
```bash
./bin/erion-ember
```

The server also starts the gRPC endpoint on port 9090 by default.

Quick HTTP smoke checks:

```bash
curl http://localhost:8080/ready
curl http://localhost:8080/health
```

If you want an end-to-end backend example instead of `curl`, run `go run ./examples/go/raw-grpc-chat`, `go run ./examples/go/raw-http-chat`, or use `npm install && npm start` in `examples/node/raw-grpc-chat` or `examples/node/raw-http-chat`.

If you want a release-demo walkthrough with stats and metrics, run `./examples/showcase/scripts/run-demo.sh`.

### 3. Your First Cache Hit
Use HTTP for a quick manual check. For production backend integration, prefer gRPC.

**Step 1: Set a response**
```bash
curl -XPOST http://localhost:8080/v1/cache/set \
  -H 'Content-Type: application/json' \
  -d '{"prompt": "How do I bake a cake?", "response": "Preheat oven to 350..."}'
```

**Step 2: Get the cached response**
```bash
curl -XPOST http://localhost:8080/v1/cache/get \
  -H 'Content-Type: application/json' \
  -d '{"prompt": "How do I bake a cake?", "similarity_threshold": 0.8}'
```
- **Expected Result**: You should see `hit: true` immediately because the prompt matches exactly.

---

## Mastering Similarity Thresholds

The `similarity_threshold` (0.0 to 1.0) controls how strict the lexical fallback is. Exact matches do not depend on this value.

| Threshold | Strictness | Use Case |
|-----------|------------|----------|
| `0.95` | Very Strict | Use when precision is critical (e.g., code snippets). |
| `0.85` | Balanced | Best for general conversation and Q&A. |
| `0.70` | Relaxed | Useful for creative writing or broad intent matching. |

---

## Performance Tips

- **Prompt Shape Matters**: Lexical fallback works best when related prompts still share important words.
- **Pre-warming**: Loading common prompts during startup can improve early hit rates.
- **TTL Usage**: Use Short TTLs (e.g., 300s) for rapidly changing data, and long TTLs for static information like FAQs.
- **Compression**: Erion Ember uses LZ4 compression automatically. You do not need to pre-compress payloads.

---

## Production Limits

- **Single Node Only**: Release 1 has no clustering or shared state.
- **Memory Only**: Restarting the process clears the cache.
- **HTTP Payload Cap**: JSON request bodies over 8 MiB are rejected; use gRPC for larger backend payloads.
- **Capacity Planning Matters**: `CACHE_MAX_ELEMENTS`, response size, and TTL settings all affect RAM usage.
- **Best Fit**: Place Erion Ember in front of one or more chat backends that can tolerate rebuilding cache state after restarts.

Use shorter TTLs for volatile prompts and lower `CACHE_MAX_ELEMENTS` when running on smaller hosts.

---

## Integration Paths

- **Preferred gRPC path**: Use `proto/ember/v1/semantic_cache.proto` with the Go or Node raw gRPC examples.
- **HTTP compatibility path**: Use the raw HTTP examples when JSON requests are easier to wire up.
- **Manual verification path**: Use the HTTP `curl` flow or the showcase script to confirm a miss -> set -> hit cycle before rollout.

---

## Troubleshooting

### "Cache Miss" for similar text
- **Problem**: Similarity isn't being detected.
- **Solution**: Lower the `similarity_threshold`. Make sure the prompts still share core keywords, because the fallback is lexical rather than embedding-based.

### "Method Not Allowed"
- **Problem**: API returns a 405 error.
- **Solution**: Ensure you are using `POST` for `get`, `set`, and `delete` endpoints.

---

## Glossary

- **BM25**: A ranking function used to estimate the relevance of documents to a given search query.
- **Jaccard Similarity**: A statistic used for gauging the similarity and diversity of sample sets.
- **LRU Cache**: Least Recently Used; a strategy where the least used items are removed first when the cache is full.
