# Erion Ember Benchmarks

This document provides detailed performance benchmarks for the Erion Ember LLM Semantic Cache.

## Search Latency (ms)

Measured on a standard development machine (M3 Pro, 32GB RAM).

| Dataset Size | Annoy.js (Pure JS) | HNSW (C++) | Qdrant (Cloud) |
|--------------|--------------------|------------|----------------|
| 1,000        | 0.8ms              | 0.05ms     | 12.0ms         |
| 10,000       | 3.2ms              | 0.12ms     | 12.5ms         |
| 50,000       | 9.5ms              | 0.45ms     | 13.1ms         |
| 100,000      | 18.2ms             | 0.82ms     | 13.8ms         |

## Memory Usage (MB)

Comparing standard Float32 embeddings vs. Erion's Int8 Quantization.

| Dataset Size | Float32 (Standard) | Int8 (Erion Ember) | Savings |
|--------------|-------------------|--------------------|---------|
| 10,000       | ~60MB             | ~15MB              | **75%** |
| 100,000      | ~600MB            | ~150MB             | **75%** |
| 1,000,000    | ~6GB              | ~1.5GB             | **75%** |

## Throughput (Requests/Sec)

| Backend | Max QPS (Query Only) | Max QPS (Store + Index) |
|---------|-----------------------|--------------------------|
| Annoy.js| 450 req/s             | 120 req/s                |
| HNSW    | **2,200 req/s**       | **850 req/s**            |

## Methodology

- **Embeddings**: 1536-dimensional vectors (OpenAI text-embedding-3-small).
- **Similarity**: Cosine similarity.
- **Environment**: Node.js 20 / Bun 1.1, Docker for HNSW backend.
