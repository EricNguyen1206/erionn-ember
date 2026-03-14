# Erion Ember

![Erion Ember Logo](assets/logo-horizontal.svg)

High-performance semantic cache service for LLM applications. Deployable as a standalone binary - like Redis, but for LLM responses. Erion Ember acts as a persistent, radiant "ember" for your intelligence layer, providing micro-second speed and semantic awareness.

## Features

- **Fast & Slow Path Caching**: Sub-microsecond exact matching via `xxhash`, with namespace-aware semantic fallback when an embedder is configured.
- **Exact-Only by Default**: The server runs without ONNX assets unless you explicitly configure a local embedder.
- **Efficient Storage**: Transparent LZ4 compression of all cached data.
- **Thread-Safe**: Built in Go with high-concurrency memory management.
- **gRPC-First Transport**: gRPC is the primary protocol; HTTP/JSON remains available for compatibility and operational tooling.
- **No Bundled SDKs**: The repository only ships the core server implementation and handlers.

## Installation

### From Source
```bash
git clone https://github.com/EricNguyen1206/erion-ember
cd erion-ember
make build
```

### Using Docker
```bash
docker-compose up -d
```

## Quick Start

1. **Start the server:**
   ```bash
   ./bin/erion-ember
   ```

2. **Cache a response over HTTP compatibility mode:**
   ```bash
   curl -XPOST http://localhost:8080/v1/cache/set \
     -H 'Content-Type: application/json' \
     -d '{"namespace":{"model":"llama3.1-8b","tenant_id":"tenant-a","system_prompt_hash":"sys-123"},"prompt":"What is Go?","response":"Go is a compiled language.","ttl":3600}'
   ```

3. **Retrieve with semantic similarity from the same namespace:**
   ```bash
   curl -XPOST http://localhost:8080/v1/cache/get \
     -H 'Content-Type: application/json' \
     -d '{"namespace":{"model":"llama3.1-8b","tenant_id":"tenant-a","system_prompt_hash":"sys-123"},"prompt":"Tell me about Go","similarity_threshold":0.8}'
   ```

## Usage

Erion Ember acts as a middleware for your LLM calls. Check the cache first; if it's a miss, call your LLM and then set the cache.

See the [User Guide](docs/USER_GUIDE.md) for detailed workflows.

## API Reference

| Transport | Method | Description |
|----------|--------|-------------|
| gRPC | `Get`, `Set`, `Delete`, `Stats`, `Health` | Primary API; all cache operations are namespace aware. |
| HTTP | `POST /v1/cache/get` | Compatibility endpoint for namespace-aware lookup. |
| HTTP | `POST /v1/cache/set` | Compatibility endpoint for namespace-aware writes. |
| HTTP | `POST /v1/cache/delete` | Compatibility endpoint for namespace-aware deletes. |
| HTTP | `GET /v1/stats` | Global cache stats; not namespace scoped. |
| HTTP | `GET /health` | Health check. |

Detailed documentation: [API Reference](docs/API_REFERENCE.md)

## Configuration

Settings can be tuned via `.env` or environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `HTTP_PORT` | `8080` | HTTP compatibility listener |
| `GRPC_PORT` | `9090` | Primary gRPC listener |
| `CACHE_SIMILARITY_THRESHOLD` | `0.85` | Default similarity threshold (0.0-1.0) |
| `CACHE_MAX_ELEMENTS` | `100000` | LRU capacity limit |
| `CACHE_DEFAULT_TTL` | `3600` | Default record TTL (seconds) |
| `CACHE_EMBEDDER_BACKEND` | _unset_ | Optional embedder backend; only `onnx` is supported today |
| `CACHE_EMBEDDING_MODEL_PATH` | _unset_ | Required to enable semantic fallback |
| `CACHE_EMBEDDING_TOKENIZER_PATH` | _unset_ | Required with ONNX mode |
| `CACHE_ONNXRUNTIME_SHARED_LIBRARY_PATH` | _unset_ | Required with ONNX mode |
| `CACHE_EMBEDDING_MAX_LENGTH` | `512` | ONNX tokenizer/model max sequence length |
| `CACHE_EMBEDDING_DIMENSION` | `384` | Expected embedding dimension |
| `CACHE_EMBEDDING_OUTPUT_NAME` | `sentence_embedding` auto-detected | Optional ONNX output override |
| `CACHE_EMBEDDING_POOLING` | `mean` | ONNX pooling mode |
| `CACHE_EMBEDDING_NORMALIZE` | `true` | Normalize embedding vectors |
| `CACHE_EMBEDDING_INTRA_OP_THREADS` | `0` | ONNX Runtime intra-op threads |
| `CACHE_EMBEDDING_INTER_OP_THREADS` | `0` | ONNX Runtime inter-op threads |

If you do not set `CACHE_EMBEDDING_MODEL_PATH`, the server starts in `exact-only` mode and skips semantic fallback entirely. If you set any ONNX-related environment variables, you must provide a complete ONNX configuration or startup will fail.

## Troubleshooting

- **Low Hit Rate**: Try lowering `similarity_threshold` to `0.75`, and confirm you are reading and writing within the same namespace.
- **Port Conflict**: Change `HTTP_PORT` in your `.env` file.
- **Memory Pressure**: Reduce `CACHE_MAX_ELEMENTS` to fit available RAM.
- **Semantic Search Disabled**: If results are exact-match only, verify the ONNX model, tokenizer, and `CACHE_ONNXRUNTIME_SHARED_LIBRARY_PATH` are all configured.

## Brand Identity

Erion Ember's visual identity is built around the "Core Ember"—a radiant spark of intelligence within a structured data hexagon. 

- **Primary Color**: Amber (`#f59e0b`)
- **Typography**: Inter (UI), Source Serif 4 (Technical), JetBrains Mono (Code)

See the full [Brand Identity Guide](assets/IDENTITY_GUIDE.md) and [Color Palette](assets/palette.md) for more details.

## Contributing

Contributions are welcome! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for local setup and development guidelines.

## License

MIT — see [LICENSE](LICENSE)
