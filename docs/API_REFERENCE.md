# Erion Ember API Reference

The Erion Ember API follows simple REST/JSON patterns. All requests and responses use the `application/json` content type.

## `POST /v1/cache/get`

Retrieve a response from the cache using semantic similarity.

### Parameters

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `prompt` | string | Yes | The query string to look up. |
| `similarity_threshold` | float | No | Override global similarity threshold (0.0–1.0). |

### Example Request

```bash
curl -XPOST http://localhost:8080/v1/cache/get \
  -H 'Content-Type: application/json' \
  -d '{"prompt": "What is Go?", "similarity_threshold": 0.85}'
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
| `400` | Bad Request | Ensure `prompt` is provided in the JSON body. |
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
  -d '{"prompt": "What is Go?"}'
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

### Example Response

```json
{
  "status": "ok"
}
```
