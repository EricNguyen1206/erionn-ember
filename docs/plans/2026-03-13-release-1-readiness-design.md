# Release 1 Readiness Design

## Status
- Final release-1 story updated on `2026-03-14` after tasks 1-7 landed in the worktree.

## Release 1 Story
- Erion Ember release 1 is a self-hosted semantic cache for chat backends.
- It runs as a single node with in-memory storage only.
- The runtime keeps the exact-match fast path first and uses BM25 + Jaccard as the lexical fallback.
- gRPC is the preferred integration surface, with the public contract published at `proto/ember/v1/semantic_cache.proto`.
- HTTP remains supported for compatibility, smoke tests, demos, and simpler JSON-based integrations.

## Production Shape
- One process, one node, no persistence.
- Cache contents are lost on restart.
- Capacity is bounded in memory by `CACHE_MAX_ELEMENTS` and average response size.
- TTL and LRU eviction are the main controls for memory pressure.
- This release is aimed at backend-side prompt/response reuse, not clustered or durable caching.

## Release Docs Index
- `README.md` - product overview, quickstart, production limits, and validation commands.
- `docs/USER_GUIDE.md` - operator guidance, first-run workflow, and integration paths.
- `docs/API_REFERENCE.md` - gRPC and HTTP contract details.
- `examples/showcase/README.md` - guided demo for a release-ready miss -> hit flow.
- `examples/go/raw-grpc-chat/main.go` and `examples/node/raw-grpc-chat/index.js` - preferred raw gRPC integrations.
- `examples/go/raw-http-chat/main.go` and `examples/node/raw-http-chat/index.js` - HTTP compatibility examples.

## Validation Expectations
- Build from source with `make build`.
- Run `go test ./...`.
- Run `go test -race ./...`.
- Run `golangci-lint run ./...`.
- Manually verify at least one end-to-end example flow and the showcase script path before release.

## Notes
- Keep docs explicit that release 1 is memory-only and single-node.
- Keep docs explicit that gRPC is preferred, not required.
- Keep command examples aligned with the current binary, ports, proto path, and examples directory layout.
