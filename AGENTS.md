# AGENTS.md

Guidance for coding agents working in `/Users/ericnguyen/DATA/Workspace/Backend/go/erionn-ember`.

## Purpose
- This file is the operating manual for agentic coding tools in this repository.
- Use it alongside the code, `README.md`, `Makefile`, and the docs in `docs/`.
- Prefer the current Go implementation over assumptions from generic Redis clones or old semantic-cache history.

## Project Snapshot
- Language: Go
- Module: `github.com/EricNguyen1206/erion-ember`
- Go version: `1.23.4`
- Primary binary: `bin/erion-ember`
- Entry point: `cmd/server/main.go`
- Service role: in-memory data cache with gRPC as the main protocol
- Public surfaces: gRPC data API plus HTTP admin endpoints
- Supported data types: strings, hashes, lists, sets
- Pub/sub: in-memory realtime delivery for online subscribers only

## Rule Files
- `.cursorrules`: not present
- `.cursor/rules/`: not present
- `.github/copilot-instructions.md`: not present
- If any of those files are added later, merge their instructions into this file and resolve conflicts explicitly.

## Source of Truth
- Trust the Go code first, then `README.md`, `docs/API_REFERENCE.md`, `docs/ARCHITECTURE.md`, and `Makefile`.
- Keep the repository focused on the cache server.
- Do not reintroduce semantic-cache logic, RESP parsing, or HTTP data routes.

## Repository Map
- `cmd/server/main.go`: config loading, startup, shutdown, shared dependency wiring
- `internal/server/grpc.go`: gRPC handlers and gRPC status mapping
- `internal/server/http.go`: `/health`, `/ready`, `/metrics`
- `internal/store/entry.go`: entry model, key types, store stats
- `internal/store/store.go`: generic key operations and TTL logic
- `internal/store/strings.go`: string commands
- `internal/store/hashes.go`: hash commands
- `internal/store/lists.go`: list commands
- `internal/store/sets.go`: set commands
- `internal/pubsub/hub.go`: pub/sub subscription registry and fan-out
- `proto/ember/v1/cache.proto`: public gRPC contract
- `examples/go/raw-grpc-cache/main.go`: simple cache example
- `examples/go/raw-grpc-pubsub/main.go`: simple pub/sub example

## Build Commands
- Main build: `make build`
- Direct binary build: `go build -o bin/erion-ember ./cmd/server/`
- Compile check entrypoint only: `go build ./cmd/server`
- Compile all packages: `go build ./...`
- Generate protobuf bindings: `make proto`
- Run from source via Make: `make run`
- Run compiled binary: `./bin/erion-ember`
- Clean artifacts: `make clean`

## Test Commands
- Full suite: `make test`
- Direct full suite: `go test ./...`
- Verbose suite: `make test-verbose`
- Race detector: `make test-race`
- One package: `go test ./internal/store`
- Fresh package run: `go test ./internal/store -count=1`
- Single store test: `go test ./internal/store -run TestStringSetGetAndOverwrite -count=1`
- Single pubsub test: `go test ./internal/pubsub -run TestHubPublishDeliversToActiveSubscribers -count=1`
- Single gRPC test: `go test ./internal/server -run TestGRPCServiceDataTypesAndTTL -count=1`
- Single server integration test: `go test ./cmd/server -run TestPublicProtoClientAgainstRunningServer -count=1`
- Multiple tests by regex: `go test ./internal/server -run 'TestGRPCServiceStringFlow|TestGRPCServiceValidation' -count=1`
- When changing shared behavior, run focused package tests first, then `go test ./...`.

## Lint and Format
- Format changed Go files with `gofmt -w` before finishing.
- If imports changed and `goimports` is available, run `goimports -w` on touched files.
- Lint target: `make lint`
- Direct lint: `golangci-lint run ./...`
- If `golangci-lint` is unavailable, say so explicitly in the final handoff.
- Do not hand-format code against `gofmt` output.

## Runtime and Configuration
- Default HTTP port: `8080`
- Default gRPC port: `9090`
- Main env vars: `HTTP_PORT`, `GRPC_PORT`
- HTTP is admin-only; do not add data commands back to it without an explicit task.
- Local container startup: `docker-compose up -d`

## Architecture
- Keep HTTP handlers thin: health, readiness, metrics only.
- Keep gRPC handlers thin: validate, delegate, translate errors.
- Keep keyspace behavior inside `internal/store`.
- Keep pub/sub behavior inside `internal/pubsub`.
- Keep startup and shutdown orchestration inside `cmd/server`.
- Share one `store.Store` and one `pubsub.Hub` across transports.

## Code Style
- Follow standard Go conventions first.
- Prefer small focused functions over broad multi-purpose helpers.
- Prefer early returns for invalid input and error paths.
- Use concrete structs for config, store state, hub state, and payloads.
- Keep exported APIs compact and hide repetitive details behind unexported helpers.
- Add comments only when logic is non-obvious, especially around range normalization, TTL, or pub/sub cleanup.

## Imports
- Use standard library imports first, blank line, then module imports.
- Let `gofmt` or `goimports` sort imports.
- Remove unused imports immediately.
- Avoid alias imports unless there is a real collision or a clear readability gain.

## Formatting and Structure
- `gofmt` output is canonical.
- Do not hand-align assignments, fields, or comments.
- Prefer one package with a few clear files over extra abstraction layers.
- Keep request and response structs close to their handlers unless reuse becomes real.
- Prefer readable switches and helper functions over reflection or clever generic plumbing.

## Types and APIs
- Use `context.Context` on public gRPC methods.
- Use `time.Duration` inside Go APIs for TTL and timing.
- Use `int64` in stats and proto counters.
- Prefer `(value, bool, error)` or `(value, error)` for store lookups where absence is not exceptional.
- Keep gRPC method names aligned with the current proto command names.
- Missing data should usually be represented in the response body, not as an error.

## Naming
- Exported identifiers use clear PascalCase names.
- Unexported helpers use concise camelCase names.
- Keep receiver names short and consistent, such as `s` for store/server and `h` for hub/handler.
- Prefer domain names like `key`, `field`, `member`, `channel`, `payload`, `entry`, and `ttl`.
- Tests should follow `TestXxx`; benchmarks should follow `BenchmarkXxx`.

## Error Handling
- Return errors as the last return value.
- Wrap underlying errors with `%w` when adding context.
- Map invalid request data to `codes.InvalidArgument` in gRPC handlers.
- Map wrong-type operations to `codes.FailedPrecondition`.
- Treat corrupted in-memory state as `codes.Internal`.
- Do not ignore meaningful errors in store or pub/sub code.

## Concurrency and Performance
- Preserve locking discipline in `store.Store` and `pubsub.Hub`.
- Keep lock scopes short and obvious.
- Preserve lazy expiration behavior unless the task explicitly changes it.
- Keep pub/sub buffers bounded; do not add unbounded backlog growth.
- Prefer simple, allocation-aware code in hot paths like list/set operations and publish fan-out.

## Testing Conventions
- Prefer table-driven tests when they improve readability.
- Add targeted tests for each new command or edge case.
- Add regression tests for wrong-type behavior, TTL behavior, and pub/sub lifecycle issues.
- Use `t.Fatal` for setup failures and `t.Errorf` or `t.Fatalf` for assertion failures.
- Keep tests direct and easy to explain; avoid overly smart helpers.

## Editing Checklist
- Check whether docs need updates in `README.md`, `docs/API_REFERENCE.md`, `docs/ARCHITECTURE.md`, or `AGENTS.md`.
- Keep Docker and runtime assumptions aligned with `cmd/server/main.go`.
- Avoid adding dependencies unless they clearly improve correctness or maintainability.
- Do not casually change gRPC method names or field names in `proto/ember/v1/cache.proto`.
- Never overwrite unrelated user changes in the working tree.

## Before Finishing
- Run `gofmt -w` on every modified Go file.
- Run focused tests for touched packages.
- Run `go test ./...` unless the change is docs-only.
- Run `golangci-lint run ./...` if available.
- Run `go mod tidy` after removing dependencies or packages.
- Report unavailable tools, skipped validation, or known follow-up work explicitly.
