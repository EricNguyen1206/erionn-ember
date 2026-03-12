# AGENTS.md

Guidance for coding agents working in `E:\Workspace\Backend\go\erionn-ember`.

## Purpose
- This file is the local operating manual for agentic coding tools.
- Use it together with the codebase, `README.md`, `CONTRIBUTING.md`, `Makefile`, and CI workflow.
- Prefer the current Go implementation over stale references in older tooling files.

## Project Snapshot
- Language: Go
- Module: `github.com/EricNguyen1206/erion-ember`
- Go version: `1.23.4`
- Entry point: `cmd/server/main.go`
- Binary target: `bin/erion-ember`
- Service role: standalone semantic cache for LLM prompt/response pairs

## Rule Files
- `.cursorrules`: not present
- `.cursor/rules/`: no files found
- `.github/copilot-instructions.md`: not present
- If any of those files are added later, merge their instructions into this file and resolve conflicts explicitly.

## Source of Truth Notes
- Trust the Go code, `README.md`, `CONTRIBUTING.md`, `Makefile`, and `.github/workflows/docker-publish.yml`.
- The repository intentionally contains only the core semantic cache plus HTTP and gRPC server logic; do not reintroduce SDK/client packages.
- Keep `cmd/server` thin; put business logic in `internal/cache` or `internal/server`.

## Repository Map
- `cmd/server/main.go`: env loading, config parsing, startup, graceful shutdown
- `internal/server/http.go`: HTTP routes, JSON request/response types, handlers, `writeJSON`
- `internal/server/grpc.go`: gRPC service contract, request/response messages, and server wrapper
- `internal/cache/semantic.go`: public cache API, stats, exact and semantic lookup orchestration
- `internal/cache/metadata.go`: metadata storage, LRU behavior, TTL, locking
- `internal/cache/scorer.go`: BM25 + Jaccard scoring, token statistics
- `internal/cache/*_test.go`: unit tests and benchmarks for cache internals
- `docker-compose.yml`: local container runtime defaults

## Build Commands
- Build the main binary: `make build`
- Direct build: `go build -o bin/erion-ember ./cmd/server/`
- Package-only compile check: `go build ./cmd/server`
- CI-style build: `go build -ldflags="-s -w" -o erion-ember ./cmd/server/`
- Run built binary: `./bin/erion-ember`
- Build and run in one step: `make run`
- Clean build artifacts: `make clean`

## Test Commands
- Full test suite: `make test`
- Direct full suite: `go test ./...`
- Verbose suite: `make test-verbose`
- Race detector: `make test-race`
- Test one package: `go test ./internal/cache`
- Test one file's package with fresh results: `go test ./internal/cache -count=1`
- Run a single test: `go test ./internal/cache -run TestNormalize -count=1`
- Run multiple tests by regex: `go test ./internal/cache -run 'TestCompress|TestNormalize' -count=1`
- Run subtests by regex: `go test ./internal/cache -run 'TestNormalize/collapses-whitespace' -count=1`
- Run one benchmark only: `go test ./internal/cache -bench BenchmarkNormalizer -run '^$'`
- Run all benchmarks in a package: `go test ./internal/cache -bench . -run '^$'`
- When changing hot-path cache logic, run package tests first, then `go test ./...`.

## Lint and Format
- Format changed Go files with `gofmt -w` before finishing.
- If import blocks changed and `goimports` is available, run `goimports -w` too.
- Lint target: `make lint`
- Direct lint: `golangci-lint run ./...`
- If `golangci-lint` is unavailable, say so clearly in the final report.

## Runtime and Environment
- Default HTTP port is `8080`.
- Default gRPC port is `9090`.
- Main env vars: `HTTP_PORT`, `GRPC_PORT`, `CACHE_SIMILARITY_THRESHOLD`, `CACHE_MAX_ELEMENTS`, `CACHE_DEFAULT_TTL`.
- `CACHE_DEFAULT_TTL` is expressed in seconds in env/config parsing and converted to `time.Duration`.
- Keep env var names and defaults aligned across `cmd/server/main.go`, `README.md`, and `docker-compose.yml`.
- Local container startup: `docker-compose up -d`

## Architecture Guidance
- Keep HTTP handlers thin: decode, validate, delegate, encode.
- Keep gRPC handlers thin: validate, delegate, translate errors.
- Prefer putting cache behavior, scoring, normalization, compression, and storage rules in `internal/cache`.
- Keep startup and shutdown wiring in `cmd/server`; avoid pushing domain logic into `main.go`.
- Preserve current HTTP routes, JSON field names, and gRPC method names unless the task explicitly changes the public API.
- Maintain the exact/semantic lookup split: fast hash lookup first, token scoring second.

## Go Style Expectations
- Follow standard Go conventions first; this repository is intentionally conventional.
- Favor small focused functions over large multi-purpose functions.
- Prefer early returns for invalid input and error paths.
- Keep exported APIs compact and move internal detail behind unexported helpers.
- Preserve useful doc comments on exported types and functions.
- Add comments only when logic is non-obvious, especially around scoring, TTL, concurrency, or performance tradeoffs.

## Imports
- Use normal Go import grouping: standard library first, blank line, then module imports.
- Keep imports sorted by tooling, not by hand.
- Remove unused imports immediately.
- Avoid alias imports unless there is a genuine name collision or readability need.

## Formatting and Structure
- `gofmt` output is the canonical style.
- Do not hand-align fields or assignments.
- Preserve concise struct definitions and straightforward constructors.
- Prefer readable helper methods over deep nesting.
- Avoid needless abstraction layers in a small codebase.
- Keep JSON request and response structs close to the handlers that use them unless reuse becomes real.

## Types and APIs
- Prefer concrete structs for config, request/response payloads, and cache state.
- Use `context.Context` on public cache operations.
- Use `time.Duration` for TTL and timing inside Go APIs.
- Use `float32` for similarity scores to match existing cache/scoring code.
- Use `int64` for counters and stats fields where the code already does so.
- Prefer `(value, bool)` for lookups and reserve `error` for meaningful failures.
- Preserve current public method shapes such as `Get`, `Set`, `Delete`, and `Stats` unless a task requires API change.

## Naming
- Exported identifiers should use clear PascalCase names.
- Unexported helpers should use concise camelCase names.
- Keep receiver names short and consistent, such as `c`, `s`, or `n`.
- Prefer domain terms like `prompt`, `response`, `threshold`, `entry`, and `tokens` over vague names.
- Tests should follow `TestXxx`; benchmarks should follow `BenchmarkXxx`.

## Error Handling
- Return errors as the last return value.
- Wrap underlying errors with `%w` when adding context.
- Validate malformed client input early and return HTTP 400 from handlers.
- Log server-side failures with `slog` before returning HTTP 500.
- Do not ignore meaningful errors in core logic.
- The existing `writeJSON` helper intentionally ignores encoder failure after headers are written; keep that tradeoff in mind before changing it.

## Concurrency and Performance
- Preserve locking discipline in `MetadataStore` and `Scorer`.
- Preserve atomic counters in `SemanticCache` stats.
- Be careful with TTL, eviction, and document-frequency updates; those paths must stay internally consistent.
- Favor allocation-aware code in normalization, compression, tokenization, and scoring hot paths.
- If you add shared mutable state, document ownership and synchronization clearly.
- If you change scoring or cache semantics, add tests and benchmarks for the regression surface.

## Testing Conventions
- `internal/cache` tests currently use the external test package form `package cache_test`; keep that unless internal access is necessary.
- Prefer table-driven tests for normalization, tokenization, scoring, and TTL behavior.
- Use `t.Fatal` for setup failures and `t.Error` or `t.Errorf` for case-level assertions.
- Add benchmark coverage for hot-path changes.
- When changing public HTTP behavior, add handler-level coverage if practical.

## Editing Checklist
- Check whether docs also need updates in `README.md`, `CONTRIBUTING.md`, or `BENCHMARKS.md`.
- Keep Docker, CI, and local build assumptions aligned if binaries, env vars, or entrypoints change.
- Avoid adding dependencies unless they clearly improve correctness or performance.
- Do not casually change public routes, JSON names, or default config values.

## Before Finishing
- Run `gofmt -w` on every modified Go file.
- Run focused tests for touched packages.
- Run `go test ./...` unless the change is docs-only.
- Run `golangci-lint run ./...` if available.
- Report any unavailable tools, skipped validation, or known follow-up work explicitly.
