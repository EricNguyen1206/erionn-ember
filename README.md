# Erion Ember

![Erion Ember Logo](assets/logo-horizontal.svg)

High-performance semantic cache service for chat backends. Release 1 is a self-hosted, single-node, memory-only cache that checks exact matches first and then falls back to lexical similarity with BM25 + Jaccard.

## Release 1 Docs

- [User Guide](docs/USER_GUIDE.md) - quickstart, operating model, and rollout advice.
- [API Reference](docs/API_REFERENCE.md) - gRPC and HTTP request/response details.
- [Architecture](docs/ARCHITECTURE.md) - internal structure and request flow.
- [Showcase Demo](examples/showcase/README.md) - guided miss -> hit release demo.

## Features

- **Exact-First Retrieval**: Fast `xxhash` lookup before lexical similarity fallback with BM25 + Jaccard.
- **Zero Dependencies**: No model files, no vector databases, no CGO, and no Python required.
- **Memory-Only Runtime**: Single-node in-memory storage with TTL and LRU eviction.
- **Efficient Storage**: Transparent LZ4 compression of cached payloads in memory.
- **Thread-Safe**: Built in Go with high-concurrency memory management.
- **Dual Protocols**: gRPC is the preferred integration path; HTTP/JSON remains supported.
- **No Bundled SDKs**: Release 1 ships the core server and protocol surfaces only.

## Installation

### From Source
```bash
git clone https://github.com/EricNguyen1206/erion-ember
cd erion-ember
make build
```

### Using Docker
```bash
docker compose up -d
```

The published container image runs as a non-root user and exposes the same default ports and cache settings as the source build.

## Quick Start

1. **Build and start the server:**
   ```bash
   make build
   ./bin/erion-ember
   ```

2. **Confirm the HTTP service is live and ready:**
   ```bash
   curl http://localhost:8080/ready
   curl http://localhost:8080/health
   ```

3. **Cache a response over HTTP:**
   ```bash
   curl -XPOST http://localhost:8080/v1/cache/set \
     -H 'Content-Type: application/json' \
     -d '{"prompt": "What is Go?", "response": "Go is a compiled language.", "ttl": 3600}'
   ```

4. **Retrieve with exact-match confirmation:**
   ```bash
   curl -XPOST http://localhost:8080/v1/cache/get \
     -H 'Content-Type: application/json' \
     -d '{"prompt": "What is Go?", "similarity_threshold": 0.8}'
   ```

For backend integrations, prefer gRPC and the public proto at `proto/ember/v1/semantic_cache.proto`.

## Usage

Erion Ember sits in front of your chat backend. Check the cache first; on a miss, call your LLM and then store the prompt/response pair.

For release 1, prefer gRPC for backend-to-backend integration. HTTP stays available for local testing, simple deployments, and environments where JSON is easier to wire up.

Current semantic behavior is intentionally simple and explicit: exact prompt match first, then lexical similarity scoring with BM25 + Jaccard when the prompt text overlaps enough to clear the threshold.

See the [User Guide](docs/USER_GUIDE.md) for detailed workflows.
If you want copy-pasteable raw integrations without an SDK, start with `examples/go/raw-grpc-chat/main.go`, `examples/go/raw-http-chat/main.go`, `examples/node/raw-grpc-chat/index.js`, or `examples/node/raw-http-chat/index.js`.
For a release-demo sandbox that shows miss -> hit plus changing stats and metrics, run `./examples/showcase/scripts/run-demo.sh` and see `examples/showcase/README.md`.

## API Reference

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/v1/cache/get` | POST | Retrieve a cached response over HTTP (exact match first, then lexical fallback) |
| `/v1/cache/set` | POST | Store a prompt/response pair |
| `/v1/cache/delete` | POST | Remove an entry |
| `/v1/stats` | GET | View hit rates and cache statistics |
| `/metrics` | GET | Prometheus-style request and cache metrics |
| `/health` | GET | Health check |
| `/ready` | GET | Readiness check |

Preferred gRPC service: `ember.v1.SemanticCacheService` with `Get`, `Set`, `Delete`, `Stats`, and `Health` methods.

Detailed documentation: [API Reference](docs/API_REFERENCE.md)

## Production Limits

Release 1 is intentionally narrow:

- Self-hosted only.
- Single node only.
- Memory-only storage with no persistence across restarts.
- gRPC is the preferred application integration path; HTTP stays available for compatibility and manual testing.
- HTTP JSON request bodies are capped at 8 MiB; use gRPC when your prompts or responses may exceed that.
- Best fit is backend-side caching for chat prompts and responses, not long-term storage or multi-node coordination.

Use `CACHE_MAX_ELEMENTS` and `CACHE_DEFAULT_TTL` to keep memory growth within the limits of your host.

## Integration Examples

- Go gRPC: `go run ./examples/go/raw-grpc-chat`
- Go HTTP: `go run ./examples/go/raw-http-chat`
- Node gRPC: `cd examples/node/raw-grpc-chat && npm install && npm start` (`Node >= 18`)
- Node HTTP: `cd examples/node/raw-http-chat && npm install && npm start` (`Node >= 18`)
- Public proto: `proto/ember/v1/semantic_cache.proto`

For a visual release walkthrough, see the showcase demo in `examples/showcase/README.md`.

## Configuration

Settings can be tuned via `.env` or environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `HTTP_PORT` | `8080` | Server listen port |
| `GRPC_PORT` | `9090` | gRPC listen port |
| `CACHE_SIMILARITY_THRESHOLD` | `0.85` | Default similarity threshold (0.0–1.0) |
| `CACHE_MAX_ELEMENTS` | `100000` | LRU capacity limit |
| `CACHE_DEFAULT_TTL` | `3600` | Default record TTL (seconds) |

Operators can scrape `GET /metrics` to observe request counts, request latency, current cache size, and cache hit/miss totals.

## Release Validation

Run the release checklist locally before cutting a tag or relying on the Docker publish workflow:

```bash
make release-validate
make docker-build
```

Equivalent commands:

```bash
go build -o bin/erion-ember ./cmd/server/
go test ./...
go test -race ./...
golangci-lint run ./...
docker build -t erion-ember:local .
```

The published container image also runs as a non-root user by default for safer self-hosted deployment.

For a near-one-command release demo, run `./examples/showcase/scripts/run-demo.sh`.

## Troubleshooting

- **Low Hit Rate**: Try lowering `similarity_threshold` to `0.75`.
- **Port Conflict**: Change `HTTP_PORT` in your `.env` file.
- **Memory Pressure**: Reduce `CACHE_MAX_ELEMENTS` to fit available RAM.

## Brand Identity

Erion Ember's visual identity is built around the "Core Ember"—a radiant spark of intelligence within a structured data hexagon. 

- **Primary Color**: Amber (`#f59e0b`)
- **Typography**: Inter (UI), Source Serif 4 (Technical), JetBrains Mono (Code)

See the full [Brand Identity Guide](assets/IDENTITY_GUIDE.md) and [Color Palette](assets/palette.md) for more details.

## Contributing

Contributions are welcome! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for local setup and development guidelines.

## License

MIT — see [LICENSE](LICENSE)
