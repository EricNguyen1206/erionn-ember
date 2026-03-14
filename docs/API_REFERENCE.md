# Erion Ember API Reference

![Erion Ember Logo](../assets/logo-horizontal.svg)

Release 1 exposes the same self-hosted, single-node, memory-only semantic cache for chat backends over gRPC and HTTP. Prefer gRPC for backend integration; use HTTP when JSON is the simpler fit for your environment. The cache checks exact prompt matches first and then falls back to lexical similarity scoring with BM25 + Jaccard.

## Quickstart Links

- Public proto: `proto/ember/v1/semantic_cache.proto`
- Go raw examples: `examples/go/raw-grpc-chat/main.go` and `examples/go/raw-http-chat/main.go`
- Node raw examples: `examples/node/raw-grpc-chat/index.js` and `examples/node/raw-http-chat/index.js`
- Showcase demo: `examples/showcase/README.md`
- Production scope: self-hosted, single-node, memory-only

## Preferred gRPC API

Proto definition: `proto/ember/v1/semantic_cache.proto`

Service: `ember.v1.SemanticCacheService`

| RPC | Request | Response | Notes |
|-----|---------|----------|-------|
| `Get` | `GetRequest` | `GetResponse` | Exact match first, then lexical fallback if needed. |
| `Set` | `SetRequest` | `SetResponse` | Stores a prompt/response pair in memory. |
| `Delete` | `DeleteRequest` | `DeleteResponse` | Removes an entry by prompt. |
| `Stats` | `StatsRequest` | `StatsResponse` | Returns cache counters and hit rate. |
| `Health` | `HealthRequest` | `HealthResponse` | Returns readiness status for the gRPC service. |

Release 1 does not include SDKs. Use the raw Go and Node examples in `examples/` as copy-pasteable integration starting points.

### Example Response

```json
{
  "status": "ready"
}
```

## HTTP API

HTTP request bodies use `application/json`. Most HTTP responses also use `application/json`, except `GET /metrics`, which returns Prometheus-style text with `text/plain; version=0.0.4`.

Release 1 keeps HTTP for compatibility and debugging, but gRPC is the preferred transport for backend-to-backend use.

HTTP JSON request bodies are capped at 8 MiB. Larger backend payloads should use gRPC instead of HTTP.

## `POST /v1/cache/get`

Retrieve a response from the cache. The server attempts an exact prompt match first and only runs the lexical fallback when needed.

### Parameters

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `prompt` | string | Yes | The query string to look up. |
| `similarity_threshold` | float | No | Override global similarity threshold (0.0–1.0) for lexical fallback. |

### Example Request

```bash
curl -XPOST http://localhost:8080/v1/cache/get \
  -H 'Content-Type: application/json' \
  -d '{"prompt": "What is Go?", "similarity_threshold": 0.85}'
```

### Example Success Response (Exact Hit)

```json
{
  "hit": true,
  "response": "Go is a compiled, statically typed language.",
  "similarity": 1,
  "exact_match": true
}
```

### Example Success Response (Miss)

```json
{
  "hit": false,
  "similarity": 0,
  "exact_match": false
}
```

### Errors

| Code | Description | Solution |
|------|-------------|----------|
| `400` | Bad Request | Ensure `prompt` is provided in the JSON body. |
| `413` | Request Entity Too Large | Keep HTTP JSON payloads under 8 MiB or switch to gRPC. |
| `405` | Method Not Allowed | Ensure you are using `POST`. |

---

## `POST /v1/cache/set`

Store a prompt/response pair in the cache.

### Parameters

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `prompt` | string | Yes | The prompt used for future lookups. |
| `response` | string | Yes | The text response to be cached. |
| `ttl` | int | No | Time-to-live in seconds (0 = default). |

### Example Request

```bash
curl -XPOST http://localhost:8080/v1/cache/set \
  -H 'Content-Type: application/json' \
  -d '{"prompt": "What is Go?", "response": "Go is a compiled language.", "ttl": 3600}'
```

### Example Response

```json
{
  "id": "1"
}
```

If `ttl` is omitted or set to `0`, the server uses `CACHE_DEFAULT_TTL`.

---

## `POST /v1/cache/delete`

Remove an entry from the cache by its original prompt.

### Parameters

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `prompt` | string | Yes | The exact prompt of the entry to delete. |

### Example Request

```bash
curl -XPOST http://localhost:8080/v1/cache/delete \
  -H 'Content-Type: application/json' \
  -d '{"prompt": "What is Go?"}'
```

### Example Response

```json
{
  "deleted": true
}
```

---

## `GET /v1/stats`

Get current cache performance statistics.

### Example Response

```json
{
  "total_entries": 1024,
  "cache_hits": 8345,
  "cache_misses": 2103,
  "total_queries": 10448,
  "hit_rate": 0.7988
}
```

---

## `GET /health`

Basic health check for the service.

This reports process health only. Release 1 has no persistence layer or cluster membership to verify.

### Example Response

```json
{
  "status": "ok"
}
```

---

## `GET /ready`

Readiness check for the in-memory service.

### Example Response

```json
{
  "status": "ready"
}
```

---

## `GET /metrics`

Prometheus-style metrics for HTTP traffic and cache behavior.

### Example Output

```text
erion_ember_cache_entries 1
erion_ember_cache_hits_total 1
erion_ember_cache_misses_total 1
erion_ember_cache_queries_total 2
erion_ember_cache_hit_rate 0.5
```
