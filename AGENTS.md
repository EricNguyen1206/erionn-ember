# AGENTS.md

Guidance for coding agents working in `/Users/ericnguyen/DATA/Workspace/Backend/go/erionn-ember`.

## Purpose
- This file is the operating manual for agentic coding tools in this repository.
- Use it alongside the code, `README.md`, `CONTRIBUTING.md`, `Makefile`, and CI workflow.
- Prefer the current Go implementation over assumptions from generic Go templates or stale tooling docs.

## Project Snapshot
- Language: Go
- Module: `github.com/EricNguyen1206/erion-ember`
- Go version: `1.23.4`
- Primary binary: `bin/erion-ember`
- Entry point: `cmd/server/main.go`
- Service role: standalone semantic cache for LLM prompt/response pairs
- Primary product context: semantic cache platform for ChatGPT-like web chat experiences; future features should stay aligned with multi-turn chat workflows, low latency, cache correctness, and production-grade operability
- Public transports: HTTP/JSON and gRPC
- Core engine: exact hash lookup first, BM25 + Jaccard semantic lookup second

## Rule Files
- `.cursorrules`: not present
- `.cursor/rules/`: not present
- `.github/copilot-instructions.md`: not present
- If any of those files are added later, merge their instructions into this file and resolve conflicts explicitly.

## Source of Truth
- Trust the Go code first, then `README.md`, `CONTRIBUTING.md`, `Makefile`, and `.github/workflows/docker-publish.yml`.
- Keep the repository focused on the cache server; do not reintroduce client SDKs or unrelated platform code.
- Keep `cmd/server` thin and place domain logic under `internal/cache` or `internal/server`.

## Repository Map
- `cmd/server/main.go`: config loading, env parsing, startup, signal handling, graceful shutdown
- `internal/cache/semantic.go`: public cache API, stats, exact lookup, semantic lookup orchestration
- `internal/cache/metadata.go`: metadata storage, LRU behavior, TTL enforcement, locking
- `internal/cache/scorer.go`: BM25 + Jaccard scoring, token statistics, document frequency updates
- `internal/cache/normalizer.go`: prompt normalization and hashing helpers
- `internal/cache/compressor.go`: LZ4 compression and decompression helpers
- `internal/server/http.go`: HTTP routes, request/response structs, JSON decode/encode helpers
- `internal/server/grpc.go`: gRPC service definitions and transport wrapper
- `internal/cache/*_test.go`: unit tests and benchmarks for cache internals
- `docker-compose.yml`: local container startup defaults

## Build Commands
- Main build: `make build`
- Direct binary build: `go build -o bin/erion-ember ./cmd/server/`
- Compile check entrypoint only: `go build ./cmd/server`
- Compile all packages: `go build ./...`
- CI-style binary build: `go build -ldflags="-s -w" -o erion-ember ./cmd/server/`
- Run from source via Make: `make run`
- Run compiled binary: `./bin/erion-ember`
- Clean artifacts: `make clean`

## Test Commands
- Full suite: `make test`
- Direct full suite: `go test ./...`
- Verbose suite: `make test-verbose`
- Race detector: `make test-race`
- One package: `go test ./internal/cache`
- Fresh package run: `go test ./internal/cache -count=1`
- Single test: `go test ./internal/cache -run TestNormalize -count=1`
- Multiple tests by regex: `go test ./internal/cache -run 'TestCompress|TestNormalize' -count=1`
- Subtest: `go test ./internal/cache -run 'TestNormalize/collapses-whitespace' -count=1`
- Benchmarks only: `go test ./internal/cache -bench . -run '^$'`
- Single benchmark: `go test ./internal/cache -bench BenchmarkNormalizer -run '^$'`
- When touching hot-path cache logic, run targeted package tests first, then `go test ./...`.

## Lint and Format
- Format changed Go files with `gofmt -w` before finishing.
- If imports changed and `goimports` is available, run `goimports -w` on the touched files.
- Lint target: `make lint`
- Direct lint: `golangci-lint run ./...`
- If `golangci-lint` is unavailable, say so explicitly in the final handoff.
- Do not hand-format code against `gofmt` output.

## Runtime and Configuration
- Default HTTP port: `8080`
- Default gRPC port: `9090`
- Main env vars: `HTTP_PORT`, `GRPC_PORT`, `CACHE_SIMILARITY_THRESHOLD`, `CACHE_MAX_ELEMENTS`, `CACHE_DEFAULT_TTL`
- `CACHE_DEFAULT_TTL` is configured in seconds and converted to `time.Duration` in startup code.
- Keep defaults aligned across `cmd/server/main.go`, `README.md`, and `docker-compose.yml`.
- Local container startup: `docker-compose up -d`

## Architecture Guidance
- Keep HTTP handlers thin: decode, validate, delegate, encode.
- Keep gRPC handlers thin: validate, delegate, translate errors.
- Keep cache behavior, normalization, scoring, compression, TTL, and eviction logic inside `internal/cache`.
- Preserve the current public HTTP routes, JSON field names, and gRPC method names unless the task explicitly changes the API.
- Preserve the fast-path exact lookup before the slow-path semantic scoring flow.
- Do not move business logic into `main.go`.

## Code Style
- Follow standard Go conventions first; this repository is intentionally conventional.
- Prefer small focused functions over broad multi-purpose helpers.
- Prefer early returns for invalid input and error paths.
- Use concrete structs for config, request/response payloads, and cache state.
- Keep exported APIs compact and hide implementation details behind unexported helpers.
- Preserve useful doc comments on exported types and functions.
- Add comments only when logic is non-obvious, especially around scoring, TTL, concurrency, or performance tradeoffs.

## Imports
- Use normal Go import grouping: standard library first, blank line, then module imports.
- Let `gofmt` or `goimports` sort imports.
- Remove unused imports immediately.
- Avoid alias imports unless there is a real name collision or readability gain.

## Formatting and Structure
- `gofmt` output is the canonical style.
- Do not hand-align assignments, fields, or comments.
- Prefer readable helpers over deep nesting.
- Preserve concise struct definitions and straightforward constructors.
- Keep request and response structs near the handlers that use them unless reuse becomes real.
- Avoid unnecessary abstraction layers in this small codebase.

## Types and APIs
- Use `context.Context` on public cache operations.
- Use `time.Duration` for TTL and timing inside Go APIs.
- Use `float32` for similarity scores to match the existing cache and scoring code.
- Use `int64` for stats and counters where the current code already does so.
- Prefer `(value, bool)` for cache lookups; use `error` for meaningful failure states.
- Preserve public method shapes such as `Get`, `Set`, `Delete`, and `Stats` unless the task requires an API change.

## Naming
- Exported identifiers should use clear PascalCase names.
- Unexported helpers should use concise camelCase names.
- Keep receiver names short and consistent, such as `c`, `h`, or `s`.
- Prefer domain names like `prompt`, `response`, `threshold`, `entry`, and `tokens` over vague names.
- Tests should follow `TestXxx`; benchmarks should follow `BenchmarkXxx`.

## Error Handling
- Return errors as the last return value.
- Wrap underlying errors with `%w` when adding context.
- Validate malformed client input early and return HTTP 400 from handlers.
- Log server-side failures with `slog` before returning HTTP 500.
- Do not ignore meaningful errors in core logic.
- Be aware that `writeJSON` intentionally ignores encoder failure after headers are written; preserve that tradeoff unless the task specifically changes response handling.

## Concurrency and Performance
- Preserve locking discipline in `MetadataStore` and `Scorer`.
- Preserve atomic counters in `SemanticCache` stats.
- Be careful with TTL eviction, LRU refresh, and document-frequency updates; those paths must stay internally consistent.
- Favor allocation-aware code in normalization, tokenization, compression, and scoring hot paths.
- If you add shared mutable state, document ownership and synchronization clearly.
- If you change scoring, cache semantics, or expiration behavior, add tests and benchmarks for the regression surface.

## Testing Conventions
- `internal/cache` tests use the external form `package cache_test`; keep that unless internal access is necessary.
- Prefer table-driven tests for normalization, tokenization, scoring, TTL, and eviction behavior.
- Use `t.Fatal` for setup failures and `t.Error` or `t.Errorf` for case-level assertions.
- Add benchmark coverage for hot-path changes.
- When changing public HTTP behavior, add handler-level coverage if practical.

## Editing Checklist
- Check whether docs also need updates in `README.md`, `CONTRIBUTING.md`, or `BENCHMARKS.md`.
- Keep Docker, CI, and local runtime assumptions aligned if binaries, env vars, or entrypoints change.
- Avoid adding dependencies unless they clearly improve correctness or performance.
- Do not casually change public routes, JSON names, default config values, or wire-level behavior.
- Never overwrite unrelated user changes in the working tree.

## Before Finishing
- Run `gofmt -w` on every modified Go file.
- Run focused tests for the touched package or packages.
- Run `go test ./...` unless the change is docs-only.
- Run `golangci-lint run ./...` if available.
- Report unavailable tools, skipped validation, or known follow-up work explicitly.
