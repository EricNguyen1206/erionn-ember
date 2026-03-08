# Changelog

All notable changes to Erion Ember will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.0.0] - 2026-03-08

### Overview
This release marks a complete reset and rewrite of Erion Ember into a standalone, pure-Go semantic caching service. It is designed for maximum performance with zero external dependencies.

### Added
- **Core Engine**: High-performance Go implementation with `xxhash` for exact matching.
- **Semantic Scoring**: Hybrid BM25 + Jaccard similarity scorer for intelligent paraphrase detection.
- **Memory Efficiency**: Transparent LZ4 compression for all cached payloads.
- **Storage**: In-memory metadata store with LRU eviction and per-entry TTL support.
- **REST API**: Standardized JSON endpoints for `get`, `set`, `delete`, and `stats`.
- **Documentation**: A complete documentation suite including:
  - Comprehensive `README.md`
  - [API Reference](docs/API_REFERENCE.md)
  - [Architecture Deep-dive](docs/ARCHITECTURE.md)
  - [User Guide](docs/USER_GUIDE.md)
  - [Contributing Guidelines](CONTRIBUTING.md)

### Infrastructure
- Multi-stage Docker build resulting in a ~20MB static binary.
- Dedicated Makefile for build, test, and linting automation.
