# Contributing to Erion Ember

![Erion Ember Logo](assets/logo-horizontal.svg)

Welcome! We're glad you're interested in contributing to Erion Ember. These guidelines will help you get started with the codebase and our development workflow.

## Local Development Setup

### Prerequisites
- **Go 1.23+**
- **Docker** (for integration tests)
- **golangci-lint** (optional, but recommended)

### Step 1: Clone and Build
```bash
git clone https://github.com/EricNguyen1206/erion-ember
cd erion-ember
make build
```

### Step 2: Running Tests
We maintain high test coverage for core logic. Always run tests before submitting a PR.
```bash
# Run all tests
make test

# Run tests with race detector
make test-race
```

## Repository Structure

- `cmd/server/`: The entry point for the HTTP service.
- `internal/cache/`: Core caching logic, scoring algorithms, and storage.
- `internal/server/`: HTTP handler implementation.
- `proto/`: Protobuf definitions (for future gRPC support).

## Development Workflow

1. **Fork and Branch**: Create a feature branch from `main`.
2. **Follow Style**: Follow standard Go conventions and see [AGENTS.md](AGENTS.md) for project-specific rules.
3. **Commit Messages**: Use descriptive commit messages (e.g., `feat: add BM25 scoring`).
4. **Pull Requests**: Open a PR with a clear description of your changes and ensure all CI checks pass.

## Coding Standards

- **Error Handling**: Return errors as the last value; never ignore them.
- **Concurrency**: Use the provided `MetadataStore` locking patterns for thread-safe state.
- **Benchmarks**: If adding performance-critical code, include benchmarks and update [BENCHMARKS.md](BENCHMARKS.md).

## Questions?
Open an issue or contact the maintainers. We're happy to help!
