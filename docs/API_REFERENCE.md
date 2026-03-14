# Erion Ember API Reference

![Erion Ember Logo](../assets/logo-horizontal.svg)

Erion Ember exposes a gRPC-first API with HTTP/JSON compatibility endpoints. All HTTP requests and responses use the `application/json` content type.

## gRPC primary API

The canonical transport is `ember.v1.SemanticCacheService` in `proto/ember/v1/semantic_cache.proto`.

- `Get`, `Set`, and `Delete` require a populated `namespace` message.
- `Stats` returns global cache counters and does not require a namespace on gRPC.
- `Health` is unchanged and has no namespace.

Namespace shape for both gRPC and HTTP compatibility requests:

```json
{
  "model": "llama3.1-8b",
  "tenant_id": "tenant-a",
  "system_prompt_hash": "sys-123"
}
```

## `POST /v1/cache/get`

Retrieve a response from the cache using exact lookup first, then semantic fallback within the supplied namespace.

### Parameters

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `namespace` | object | Yes | Namespace partition containing `model`, `tenant_id`, and `system_prompt_hash`. |
| `prompt` | string | Yes | The query string to look up. |
| `similarity_threshold` | float | No | Override global similarity threshold (0.0-1.0). |

### Example Request

```bash
curl -XPOST http://localhost:8080/v1/cache/get \
  -H 'Content-Type: application/json' \
  -d '{"namespace":{"model":"llama3.1-8b","tenant_id":"tenant-a","system_prompt_hash":"sys-123"},"prompt":"What is Go?","similarity_threshold":0.85}'
```

### Example Success Response (Hit)

```json
{
  "hit": true,
  "response": "Go is a compiled, statically typed language.",
  "similarity": 0.97,
  "exact_match": false
}
```

### Example Success Response (Miss)

```json
{
  "hit": false
}
```

### Errors

| Code | Description | Solution |
|------|-------------|----------|
| `400` | Bad Request | Ensure `namespace` and `prompt` are provided in the JSON body. |
| `405` | Method Not Allowed | Ensure you are using `POST`. |

---

## `POST /v1/cache/set`

Store a prompt/response pair in the cache.

### Parameters

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `namespace` | object | Yes | Namespace partition containing `model`, `tenant_id`, and `system_prompt_hash`. |
| `prompt` | string | Yes | The prompt used for future lookups. |
| `response` | string | Yes | The text response to be cached. |
| `ttl` | int | No | Time-to-live in seconds (0 = default). |

### Example Request

```bash
curl -XPOST http://localhost:8080/v1/cache/set \
  -H 'Content-Type: application/json' \
  -d '{"namespace":{"model":"llama3.1-8b","tenant_id":"tenant-a","system_prompt_hash":"sys-123"},"prompt":"What is Go?","response":"Go is a compiled language.","ttl":3600}'
```

### Example Response

```json
{
  "id": "1"
}
```

---

## `POST /v1/cache/delete`

Remove an entry from the cache by its original prompt within the supplied namespace.

### Parameters

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `namespace` | object | Yes | Namespace partition containing `model`, `tenant_id`, and `system_prompt_hash`. |
| `prompt` | string | Yes | The exact prompt of the entry to delete. |

### Example Request

```bash
curl -XPOST http://localhost:8080/v1/cache/delete \
  -H 'Content-Type: application/json' \
  -d '{"namespace":{"model":"llama3.1-8b","tenant_id":"tenant-a","system_prompt_hash":"sys-123"},"prompt":"What is Go?"}'
```

---

## `GET /v1/stats`

Get current cache performance statistics.

The HTTP stats endpoint is global. It does not accept a namespace and it does not return namespace-scoped counters.

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

### Example Response

```json
{
  "status": "ok"
}
```

## Runtime notes

- Without `CACHE_EMBEDDING_MODEL_PATH`, Erion Ember runs in exact-only mode.
- Semantic fallback requires a complete ONNX configuration, including `CACHE_EMBEDDING_TOKENIZER_PATH` and `CACHE_ONNXRUNTIME_SHARED_LIBRARY_PATH`.
- If the embedder or vector index cannot be used during a write, the cache still stores the entry and serves exact matches.
