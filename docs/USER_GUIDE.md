# Erion Ember User Guide

![Erion Ember Logo](../assets/logo-horizontal.svg)

Welcome to Erion Ember! This guide will help you set up and master semantic caching for your LLM applications. Erion Ember is designed to provide high-performance caching that acts like a persistent, radiant "ember" for your intelligence layer.

## What is Erion Ember?

Erion Ember is a lightning-fast semantic cache. Unlike traditional caches that require an exact string match, Erion Ember understands when two prompts have the same meaning, even if they use different words.

### Why Use It?
- **Save Costs**: Reuse LLM responses for similar queries.
- **Reduce Latency**: Cached hits return in milliseconds.
- **Privacy**: High-performance local caching with no cloud dependencies.

## Getting Started

### 1. Installation
Build the binary directly from source:
```bash
make build
```

### 2. Start the Service
Run the binary to start the server on port 8080:
```bash
./bin/erion-ember
```

### 3. Your First Cache Hit
Try setting a value and then retrieving it with a different but similar prompt.

**Step 1: Set a response**
```bash
curl -XPOST http://localhost:8080/v1/cache/set \
  -d '{"prompt": "How do I bake a cake?", "response": "Preheat oven to 350..."}'
```

**Step 2: Get via similarity**
```bash
curl -XPOST http://localhost:8080/v1/cache/get \
  -d '{"prompt": "Tell me the steps to bake a cake", "similarity_threshold": 0.8}'
```
- **Expected Result**: You should see `hit: true` with the original cake instructions.

---

## Mastering Similarity Thresholds

The `similarity_threshold` (0.0 to 1.0) controls how strict the matching is.

| Threshold | Strictness | Use Case |
|-----------|------------|----------|
| `0.95` | Very Strict | Use when precision is critical (e.g., code snippets). |
| `0.85` | Balanced | Best for general conversation and Q&A. |
| `0.70` | Relaxed | Useful for creative writing or broad intent matching. |

---

## Performance Tips

- **Pre-warming**: Loading common prompts during startup can significantly improve initial performance.
- **TTL Usage**: Use Short TTLs (e.g., 300s) for rapidly changing data, and long TTLs for static information like FAQs.
- **Compression**: Erion Ember uses LZ4 compression automatically. You don't need to pre-compress your JSON payloads.

---

## Troubleshooting

### "Cache Miss" for similar text
- **Problem**: Similarity isn't being detected.
- **Solution**: Lower the `similarity_threshold`. Ensure the prompts share at least some core keywords, as BM25 relies on term importance.

### "Method Not Allowed"
- **Problem**: API returns a 405 error.
- **Solution**: Ensure you are using `POST` for `get`, `set`, and `delete` endpoints.

---

## Glossary

- **BM25**: A ranking function used to estimate the relevance of documents to a given search query.
- **Jaccard Similarity**: A statistic used for gauging the similarity and diversity of sample sets.
- **LRU Cache**: Least Recently Used; a strategy where the least used items are removed first when the cache is full.
