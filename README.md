# Erion Ember

High-performance semantic cache service for LLM applications. Deployable as a standalone binary — like Redis, but for LLM responses.

## Features

- **Fast & Slow Path Caching**: Sub-microsecond exact matching via `xxhash` and intelligent similarity detection via BM25 + Jaccard.
- **Zero Dependencies**: No model files, no vector databases, no CGO, and no Python required.
- **Efficient Storage**: Transparent LZ4 compression of all cached data.
- **Thread-Safe**: Built in Go with high-concurrency memory management.
- **Developer Friendly**: Simple REST/JSON API for easy integration.

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

2. **Cache a response:**
   ```bash
   curl -XPOST http://localhost:8080/v1/cache/set \
     -d '{"prompt": "What is Go?", "response": "Go is a compiled language.", "ttl": 3600}'
   ```

3. **Retrieve with semantic similarity:**
   ```bash
   curl -XPOST http://localhost:8080/v1/cache/get \
     -d '{"prompt": "Tell me about Go", "similarity_threshold": 0.8}'
   ```

## Usage

Erion Ember acts as a middleware for your LLM calls. Check the cache first; if it's a miss, call your LLM and then set the cache.

See the [User Guide](docs/USER_GUIDE.md) for detailed workflows.

## API Reference

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/v1/cache/get` | POST | Retrieve a cached response (semantic search) |
| `/v1/cache/set` | POST | Store a prompt/response pair |
| `/v1/cache/delete` | POST | Remove an entry |
| `/v1/stats` | GET | View hit rates and cache statistics |
| `/health` | GET | Health check |

Detailed documentation: [API Reference](docs/API_REFERENCE.md)

## Configuration

Settings can be tuned via `.env` or environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `HTTP_PORT` | `8080` | Server listen port |
| `CACHE_SIMILARITY_THRESHOLD` | `0.85` | Default similarity threshold (0.0–1.0) |
| `CACHE_MAX_ELEMENTS` | `100000` | LRU capacity limit |
| `CACHE_DEFAULT_TTL` | `3600` | Default record TTL (seconds) |

## Troubleshooting

- **Low Hit Rate**: Try lowering `similarity_threshold` to `0.75`.
- **Port Conflict**: Change `HTTP_PORT` in your `.env` file.
- **Memory Pressure**: Reduce `CACHE_MAX_ELEMENTS` to fit available RAM.

## Contributing

Contributions are welcome! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for local setup and development guidelines.

## License

MIT — see [LICENSE](LICENSE)