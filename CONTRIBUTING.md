# Contributing to Erion Ember

## Local Setup

```bash
git clone https://github.com/EricNguyen1206/erion-ember
cd erion-ember
make build
```

Useful development commands:

```bash
make test
make test-race
make lint
make proto
```

## Project Structure

- `cmd/server/` - process startup and shutdown
- `internal/server/` - gRPC handlers and HTTP admin endpoints
- `internal/store/` - keyspace and datatype commands
- `internal/pubsub/` - subscription registry and message fan-out
- `proto/ember/v1/` - public gRPC contract

## Workflow

1. Create a feature branch.
2. Add or update tests first.
3. Keep HTTP admin-only unless the task explicitly changes that.
4. Run focused tests, then `go test ./...`.
5. Run `gofmt -w` on changed Go files.
6. Run `golangci-lint run ./...` if available.

## Style Notes

- Prefer clear, direct Go over abstraction-heavy designs.
- Keep handlers thin and put business logic in `internal/store` or `internal/pubsub`.
- Use gRPC status codes consistently:
  - `InvalidArgument` for bad input
  - `FailedPrecondition` for wrong-type operations
  - `Internal` for unexpected server-side corruption or failures

See `AGENTS.md` for the repository-specific coding rules used by coding agents.
