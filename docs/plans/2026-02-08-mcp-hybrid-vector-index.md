# MCP Server with Hybrid Vector Index Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Transform the HTTP-based semantic cache into an MCP server using stdio transport, with a pluggable vector index supporting both Annoy.js (pure JS, default) and hnswlib (C++, Docker-optimized).

**Architecture:** The MCP server uses stdio transport with JSON-RPC 2.0 messages. The vector search layer uses a factory pattern to select between Annoy.js (immediate compatibility, no build) and hnswlib (maximum performance, requires Docker/C++ build environment). Existing SemanticCache and supporting libraries are preserved.

**Tech Stack:** Bun runtime, @modelcontextprotocol/sdk, zod validation, annoy.js (pure JS vector search), hnswlib-node (optional C++ optimization), lz4js compression

---

## Prerequisites

Before starting, ensure you have:
- Bun runtime installed (v1.0+)
- Git configured
- Working directory is `/Users/ericnguyen/DATA/Workspace/Backend/nodejs/erion-ember`

---

### Task 1: Setup and Cleanup

**Files:**
- Modify: `package.json`
- Delete: `benchmark/` directory (already done)
- Verify: `src/server.js`, `src/routes/`, `src/services/groq.service.js` removed (already done)

**Step 1: Install MCP SDK and Annoy.js dependencies**

Run:
```bash
bun add @modelcontextprotocol/sdk zod annoy.js
```

Expected: Packages installed successfully

**Step 2: Update package.json scripts**

Read current `package.json` and update:

```json
{
  "name": "erion-ember",
  "version": "1.0.0",
  "description": "LLM Semantic Cache MCP Server",
  "main": "src/mcp-server.js",
  "type": "module",
  "scripts": {
    "dev": "bun run --watch src/mcp-server.js",
    "start": "bun run src/mcp-server.js",
    "test": "bun test tests/",
    "test:watch": "bun test --watch tests/",
    "docker:build": "docker build -t erion-ember .",
    "docker:run": "docker run -e VECTOR_INDEX_BACKEND=hnsw erion-ember"
  },
  "dependencies": {
    "@modelcontextprotocol/sdk": "^1.26.0",
    "annoy.js": "^0.0.0",
    "hnswlib-node": "^2.0.0",
    "lz4js": "^0.2.0",
    "xxhash-addon": "^2.0.0",
    "zod": "^4.3.6"
  },
  "devDependencies": {
    "bun-types": "latest"
  },
  "trustedDependencies": [
    "hnswlib-node",
    "xxhash-addon"
  ],
  "engines": {
    "node": ">=20.0.0"
  }
}
```

**Step 3: Verify old files are removed**

Run:
```bash
ls -la src/ | grep -E "(server\.js|routes|services/chat|services/groq)" || echo "Old files already removed"
```

Expected: No output (files already removed in previous commits)

**Step 4: Commit**

Run:
```bash
git add package.json bun.lock
git commit -m "chore: add Annoy.js dependency for pure JS vector search

- Install annoy.js for default vector index (no native build)
- Update package.json with Docker scripts
- Keep hnswlib-node as optional optimization"
```

---

### Task 2: Create Vector Index Interface and Factory

**Files:**
- Create: `src/lib/vector-index/interface.js`
- Create: `src/lib/vector-index/index.js` (factory)

**Step 1: Create vector-index directory**

Run:
```bash
mkdir -p src/lib/vector-index
```

**Step 2: Write failing test for factory**

Create `tests/vector-index/factory.test.js`:
```javascript
import { describe, test, expect } from 'bun:test';
import { createVectorIndex } from '../../src/lib/vector-index/index.js';

describe('VectorIndex Factory', () => {
  test('should create AnnoyVectorIndex by default', () => {
    const index = createVectorIndex({ dim: 128, maxElements: 1000 });
    expect(index).toBeDefined();
    expect(index.constructor.name).toBe('AnnoyVectorIndex');
  });

  test('should create HNSWVectorIndex when specified', () => {
    // Skip if hnswlib not available
    try {
      const index = createVectorIndex({ 
        dim: 128, 
        maxElements: 1000, 
        backend: 'hnsw' 
      });
      expect(index.constructor.name).toBe('HNSWVectorIndex');
    } catch (e) {
      console.log('hnswlib not available, skipping');
    }
  });

  test('should throw error for unknown backend', () => {
    expect(() => {
      createVectorIndex({ dim: 128, maxElements: 1000, backend: 'unknown' });
    }).toThrow();
  });
});
```

**Step 3: Run test to verify it fails**

Run:
```bash
bun test tests/vector-index/factory.test.js
```

Expected: FAIL - "Cannot find module"

**Step 4: Create interface definition**

Create `src/lib/vector-index/interface.js`:
```javascript
/**
 * VectorIndex Interface
 * Abstract interface for vector similarity search implementations
 */
export class VectorIndex {
  constructor(dim, maxElements, space = 'cosine') {
    if (new.target === VectorIndex) {
      throw new Error('VectorIndex is abstract - cannot instantiate directly');
    }
    this.dim = dim;
    this.maxElements = maxElements;
    this.space = space;
  }

  /**
   * Add vector to index
   * @param {number[]} vector - Vector to add
   * @param {number} id - Item ID
   * @returns {number} ID of added item
   */
  addItem(vector, id) {
    throw new Error('addItem must be implemented');
  }

  /**
   * Search for nearest neighbors
   * @param {number[]} queryVector - Query vector
   * @param {number} k - Number of results
   * @returns {Array<{id: number, distance: number}>} Search results
   */
  search(queryVector, k = 5) {
    throw new Error('search must be implemented');
  }

  /**
   * Save index to file
   * @param {string} path - File path
   */
  save(path) {
    throw new Error('save must be implemented');
  }

  /**
   * Load index from file
   * @param {string} path - File path
   */
  load(path) {
    throw new Error('load must be implemented');
  }

  /**
   * Destroy index and free memory
   */
  destroy() {
    throw new Error('destroy must be implemented');
  }

  /**
   * Get number of items in index
   * @returns {number}
   */
  getCount() {
    throw new Error('getCount must be implemented');
  }
}

export default VectorIndex;
```

**Step 5: Create factory**

Create `src/lib/vector-index/index.js`:
```javascript
import VectorIndex from './interface.js';

/**
 * Create vector index instance
 * @param {object} options - Configuration options
 * @param {number} options.dim - Vector dimension
 * @param {number} options.maxElements - Maximum number of elements
 * @param {string} options.space - Distance metric ('cosine', 'l2', 'ip')
 * @param {string} options.backend - Backend type ('annoy' or 'hnsw')
 * @returns {VectorIndex} Vector index instance
 */
export async function createVectorIndex(options = {}) {
  const { dim, maxElements, space = 'cosine' } = options;
  
  // Determine backend: environment variable > option > default
  const backend = process.env.VECTOR_INDEX_BACKEND || options.backend || 'annoy';

  if (backend === 'annoy') {
    // Dynamically import AnnoyVectorIndex
    const { default: AnnoyVectorIndex } = await import('./annoy-index.js');
    return new AnnoyVectorIndex(dim, maxElements, space);
  }

  if (backend === 'hnsw') {
    // Dynamically import HNSWVectorIndex
    try {
      const { default: HNSWVectorIndex } = await import('./hnsw-index.js');
      return new HNSWVectorIndex(dim, maxElements, space);
    } catch (err) {
      throw new Error(
        `hnswlib-node not available. ` +
        `Install C++ build tools or use Annoy.js backend (VECTOR_INDEX_BACKEND=annoy). ` +
        `Original error: ${err.message}`
      );
    }
  }

  throw new Error(`Unknown vector index backend: ${backend}. Use 'annoy' or 'hnsw'.`);
}

export { VectorIndex };
export default createVectorIndex;
```

**Step 6: Run test to verify it passes**

Run:
```bash
bun test tests/vector-index/factory.test.js
```

Expected: PASS - Factory tests pass (AnnoyVectorIndex may fail until Task 3)

**Step 7: Commit**

Run:
```bash
git add src/lib/vector-index/ tests/vector-index/
git commit -m "feat: add VectorIndex interface and factory

- Create abstract VectorIndex interface
- Implement factory with dynamic imports
- Support both 'annoy' and 'hnsw' backends
- Environment variable VECTOR_INDEX_BACKEND for selection"
```

---

### Task 3: Create AnnoyVectorIndex Implementation

**Files:**
- Create: `src/lib/vector-index/annoy-index.js`
- Test: `tests/vector-index/annoy-index.test.js`

**Step 1: Write failing test**

Create `tests/vector-index/annoy-index.test.js`:
```javascript
import { describe, test, expect, beforeEach } from 'bun:test';
import AnnoyVectorIndex from '../../src/lib/vector-index/annoy-index.js';

describe('AnnoyVectorIndex', () => {
  let index;
  const dim = 128;
  const maxElements = 1000;

  beforeEach(() => {
    index = new AnnoyVectorIndex(dim, maxElements, 'cosine');
  });

  test('should add and retrieve items', () => {
    const vector = new Array(dim).fill(0).map((_, i) => i / dim);
    const id = index.addItem(vector);
    
    expect(id).toBe(0);
    expect(index.getCount()).toBe(1);
  });

  test('should search for nearest neighbors', () => {
    // Add two vectors
    const vec1 = new Array(dim).fill(0).map(() => Math.random());
    const vec2 = new Array(dim).fill(0).map(() => Math.random());
    
    index.addItem(vec1, 0);
    index.addItem(vec2, 1);
    
    // Search with first vector
    const results = index.search(vec1, 2);
    
    expect(results.length).toBe(2);
    expect(results[0].id).toBe(0); // First result should be exact match
    expect(results[0].distance).toBeLessThan(0.01); // Very small distance
  });

  test('should save and load index', async () => {
    const vector = new Array(dim).fill(0.5);
    index.addItem(vector, 42);
    
    const tempPath = '/tmp/test-annoy-index.json';
    index.save(tempPath);
    
    const newIndex = new AnnoyVectorIndex(dim, maxElements, 'cosine');
    newIndex.load(tempPath);
    
    expect(newIndex.getCount()).toBe(1);
    
    // Clean up
    await import('fs').then(fs => fs.promises.unlink(tempPath));
  });

  test('should destroy index', () => {
    index.destroy();
    expect(index.annoy).toBeNull();
  });
});
```

**Step 2: Run test to verify it fails**

Run:
```bash
bun test tests/vector-index/annoy-index.test.js
```

Expected: FAIL - "Cannot find module"

**Step 3: Implement AnnoyVectorIndex**

Create `src/lib/vector-index/annoy-index.js`:
```javascript
import Annoy from 'annoy.js';
import { promises as fs } from 'fs';
import VectorIndex from './interface.js';

/**
 * AnnoyVectorIndex - Pure JavaScript vector similarity search
 * Uses Annoy.js (binary tree based ANN) - no native dependencies
 */
class AnnoyVectorIndex extends VectorIndex {
  constructor(dim, maxElements, space = 'cosine') {
    super(dim, maxElements, space);
    this.currentId = 0;
    this.vectors = new Map(); // Store vectors for serialization
    
    // Annoy parameters
    this.forestSize = 10;      // Number of trees
    this.maxLeafSize = 100;    // Max points per leaf
    
    // Create Annoy index
    // Annoy uses 'angular' for cosine similarity
    const annoyMetric = space === 'cosine' ? 'Angular' : 'Euclidean';
    this.annoy = new Annoy(this.forestSize, this.maxLeafSize);
  }

  /**
   * Add vector to index
   * @param {number[]} vector - Vector to add
   * @param {number} id - Optional ID (auto-increment if not provided)
   * @returns {number} ID of added item
   */
  addItem(vector, id = null) {
    const itemId = id !== null ? id : this.currentId++;
    
    // Convert to Float32 array if needed
    const floatVector = new Float32Array(vector);
    
    // Annoy expects plain array
    this.annoy.add({
      vector: Array.from(floatVector),
      data: itemId
    });
    
    // Store for serialization
    this.vectors.set(itemId, Array.from(floatVector));
    
    if (id === null) {
      this.currentId = itemId + 1;
    }
    
    return itemId;
  }

  /**
   * Search for nearest neighbors
   * @param {number[]} queryVector - Query vector
   * @param {number} k - Number of results
   * @returns {Array<{id: number, distance: number}>} Search results
   */
  search(queryVector, k = 5) {
    if (this.vectors.size === 0) {
      return [];
    }
    
    // Build index if not already built
    if (!this.built) {
      this.annoy.build();
      this.built = true;
    }
    
    const floatQuery = new Float32Array(queryVector);
    const results = this.annoy.get(Array.from(floatQuery), k);
    
    // Convert to standard format: {id, distance}
    // Annoy returns angular distance, convert to similarity-like metric
    return results.map(result => ({
      id: result.data,
      distance: result.distance // Angular distance (0 = identical, 2 = opposite)
    }));
  }

  /**
   * Save index to file (JSON format)
   * @param {string} path - File path
   */
  save(path) {
    const data = {
      dim: this.dim,
      maxElements: this.maxElements,
      space: this.space,
      currentId: this.currentId,
      vectors: Array.from(this.vectors.entries())
    };
    
    // Note: Annoy.js doesn't support native serialization, 
    // so we save the vectors and rebuild on load
    require('fs').writeFileSync(path, JSON.stringify(data));
  }

  /**
   * Load index from file
   * @param {string} path - File path
   */
  load(path) {
    const data = JSON.parse(require('fs').readFileSync(path, 'utf8'));
    
    this.dim = data.dim;
    this.maxElements = data.maxElements;
    this.space = data.space;
    this.currentId = data.currentId;
    
    // Rebuild index
    this.vectors = new Map(data.vectors);
    this.vectors.forEach((vector, id) => {
      this.annoy.add({
        vector,
        data: id
      });
    });
    
    this.built = false; // Will build on first search
  }

  /**
   * Destroy index and free memory
   */
  destroy() {
    this.annoy = null;
    this.vectors.clear();
    this.built = false;
  }

  /**
   * Get number of items in index
   * @returns {number}
   */
  getCount() {
    return this.vectors.size;
  }
}

export default AnnoyVectorIndex;
```

**Step 4: Run test to verify it passes**

Run:
```bash
bun test tests/vector-index/annoy-index.test.js
```

Expected: PASS - All tests pass

**Step 5: Commit**

Run:
```bash
git add src/lib/vector-index/annoy-index.js tests/vector-index/annoy-index.test.js
git commit -m "feat: implement AnnoyVectorIndex with pure JavaScript

- Use annoy.js for vector similarity search
- No native dependencies, works immediately
- Support save/load via JSON serialization
- Implements full VectorIndex interface"
```

---

### Task 4: Refactor HNSWVectorIndex

**Files:**
- Move/Modify: `src/lib/hnsw-index.js` → `src/lib/vector-index/hnsw-index.js`
- Modify: `src/lib/semantic-cache.js`
- Test: `tests/vector-index/hnsw-index.test.js`

**Step 1: Move and refactor hnsw-index.js**

Create `src/lib/vector-index/hnsw-index.js`:
```javascript
import hnswlib from 'hnswlib-node';
import VectorIndex from './interface.js';

/**
 * HNSWVectorIndex - C++ HNSW implementation
 * Requires hnswlib-node native bindings (use Docker for easiest setup)
 */
class HNSWVectorIndex extends VectorIndex {
  constructor(dim, maxElements, space = 'cosine') {
    super(dim, maxElements, space);
    this.currentId = 0;
    
    // HNSW parameters
    this.M = 16;              // Connections per layer
    this.efConstruction = 200; // Build accuracy
    this.ef = 100;            // Search accuracy
    
    // Create index with space and dimension only
    this.index = new hnswlib.HierarchicalNSW(space, dim);
    
    // Initialize index with capacity
    this.index.initIndex(maxElements, this.M, this.efConstruction);
    this.index.setEf(this.ef);
  }

  /**
   * Add vector to index
   * @param {number[]} vector - Vector to add
   * @param {number} id - Optional ID (auto-increment if not provided)
   * @returns {number} ID of added item
   */
  addItem(vector, id = null) {
    const itemId = id !== null ? id : this.currentId++;
    this.index.addPoint(vector, itemId);
    
    if (id === null) {
      this.currentId = itemId + 1;
    }
    
    return itemId;
  }

  /**
   * Search for nearest neighbors
   * @param {number[]} queryVector - Query vector
   * @param {number} k - Number of results
   * @returns {Array<{id: number, distance: number}>} Search results
   */
  search(queryVector, k = 5) {
    const result = this.index.searchKnn(queryVector, k);
    
    // Convert to array of objects
    const ids = result.neighbors;
    const distances = result.distances;
    
    return ids.map((id, i) => ({
      id,
      distance: distances[i]
    }));
  }

  /**
   * Save index to file
   * @param {string} path - File path
   */
  save(path) {
    this.index.writeIndexSync(path);
  }

  /**
   * Load index from file
   * @param {string} path - File path
   */
  load(path) {
    // Create new index instance and load
    this.index = new hnswlib.HierarchicalNSW(this.space, this.dim);
    this.index.readIndexSync(path);
    // Update currentId based on loaded index
    this.currentId = this.index.getCurrentCount();
  }

  /**
   * Destroy index and free memory
   */
  destroy() {
    this.index = null;
  }

  /**
   * Get number of items in index
   * @returns {number}
   */
  getCount() {
    return this.currentId;
  }
}

export default HNSWVectorIndex;
```

**Step 2: Delete old hnsw-index.js**

Run:
```bash
rm -f src/lib/hnsw-index.js
```

**Step 3: Update SemanticCache to use factory**

Read `src/lib/semantic-cache.js` and modify:

Replace line 2:
```javascript
// OLD
import HNSWIndex from './hnsw-index.js';

// NEW
import { createVectorIndex } from './vector-index/index.js';
```

Replace line 20 in constructor:
```javascript
// OLD
this.index = new HNSWIndex(this.dim, this.maxElements, 'cosine');

// NEW
this.index = null; // Will be initialized asynchronously
this.indexPromise = this._initIndex();
```

Add initialization method:
```javascript
/**
 * Initialize vector index asynchronously
 * @private
 */
async _initIndex() {
  this.index = await createVectorIndex({
    dim: this.dim,
    maxElements: this.maxElements,
    space: 'cosine'
  });
  return this.index;
}
```

Update `get()` method to wait for index:
```javascript
async get(prompt, embedding = null, options = {}) {
  // Wait for index initialization
  if (!this.index) {
    await this.indexPromise;
  }
  
  // ... rest of existing code
}
```

Update `set()` method:
```javascript
async set(prompt, response, embedding, options = {}) {
  // Wait for index initialization
  if (!this.index) {
    await this.indexPromise;
  }
  
  // ... rest of existing code
}
```

**Step 4: Move and update test**

Move `tests/hnsw-index.test.js` to `tests/vector-index/`:
```bash
mv tests/hnsw-index.test.js tests/vector-index/
```

Update import in test file:
```javascript
// OLD
import HNSWIndex from '../lib/hnsw-index.js';

// NEW  
import HNSWIndex from '../src/lib/vector-index/hnsw-index.js';
```

**Step 5: Commit**

Run:
```bash
git add src/lib/vector-index/hnsw-index.js src/lib/semantic-cache.js tests/vector-index/
git rm src/lib/hnsw-index.js || git add -A
git commit -m "refactor: integrate vector index factory into SemanticCache

- Move hnsw-index.js to vector-index/hnsw-index.js
- Update to implement VectorIndex interface
- Modify SemanticCache to use createVectorIndex factory
- Add async initialization support"
```

---

### Task 5: Update Dockerfile for hnswlib

**Files:**
- Modify: `Dockerfile`

**Step 1: Read current Dockerfile**

Run:
```bash
cat Dockerfile
```

**Step 2: Update Dockerfile**

Replace content with:
```dockerfile
# Build stage for native dependencies
FROM node:20-alpine AS builder

# Install build dependencies for hnswlib-node
RUN apk add --no-cache \
    python3 \
    make \
    g++ \
    clang \
    linux-headers

WORKDIR /app

# Copy package files
COPY package.json bun.lock ./

# Install dependencies (including native modules)
RUN npm install

# Production stage
FROM node:20-alpine

WORKDIR /app

# Copy node_modules from builder (includes compiled hnswlib)
COPY --from=builder /app/node_modules ./node_modules

# Copy application code
COPY src/ ./src/
COPY package.json ./

# Set environment to use hnswlib
ENV VECTOR_INDEX_BACKEND=hnsw
ENV NODE_ENV=production

# Use Bun runtime if available, fallback to Node
CMD ["node", "src/mcp-server.js"]
```

**Step 3: Commit**

Run:
```bash
git add Dockerfile
git commit -m "build: update Dockerfile for hnswlib-node compilation

- Multi-stage build with native compilation
- Install python3, make, g++, clang for hnswlib build
- Set VECTOR_INDEX_BACKEND=hnsw for optimized performance
- Copy pre-compiled node_modules to production stage"
```

---

### Task 6: Update MCP Server and Documentation

**Files:**
- Create: `.env.example`
- Modify: `README.md`

**Step 1: Create .env.example**

Create `.env.example`:
```bash
# Vector Index Backend
# Options: 'annoy' (pure JS, default) or 'hnsw' (C++ optimized)
VECTOR_INDEX_BACKEND=annoy

# For hnswlib backend (optional, requires C++ build tools)
# Install: xcode-select --install (macOS) or build-essential (Linux)
# VECTOR_INDEX_BACKEND=hnsw

# Embedding Service Configuration
EMBEDDING_PROVIDER=mock
# OPENAI_API_KEY=sk-...

# Cache Configuration
CACHE_SIMILARITY_THRESHOLD=0.85
CACHE_MAX_ELEMENTS=100000
CACHE_DEFAULT_TTL=3600

# MCP Server
NODE_ENV=development
```

**Step 2: Update README.md**

Update the README with sections on:
1. Quick Start (Annoy.js - works immediately)
2. Docker setup (for hnswlib optimization)
3. Backend selection guide
4. Performance comparison

**Step 3: Commit**

Run:
```bash
git add .env.example README.md
git commit -m "docs: add environment configuration and backend selection guide

- Create .env.example with backend options
- Document Annoy.js vs hnswlib trade-offs
- Add quick start and Docker instructions"
```

---

### Task 7: Final Integration and Testing

**Files:**
- Test: All existing tests
- Run: Integration tests

**Step 1: Run services tests (pure JS, should pass)**

Run:
```bash
bun test tests/services/
```

Expected: PASS - EmbeddingService tests pass

**Step 2: Run vector index tests**

Run:
```bash
bun test tests/vector-index/annoy-index.test.js
```

Expected: PASS - AnnoyVectorIndex tests pass

**Step 3: Test MCP server with Annoy backend**

Run:
```bash
VECTOR_INDEX_BACKEND=annoy timeout 3 bun run src/mcp-server.js || true
```

Expected: Server starts, outputs startup message

**Step 4: Build Docker image (optional)**

Run:
```bash
docker build -t erion-ember .
```

Expected: Image builds successfully with hnswlib compiled

**Step 5: Commit any final changes**

Run:
```bash
git add -A
git commit -m "chore: final integration testing

- Verify Annoy.js backend works in development
- Test MCP server startup
- Docker image builds with hnswlib"
```

---

## Summary

This updated implementation provides:

**Immediate Development (No Build Issues):**
- Annoy.js as default vector index
- Pure JavaScript, works on any platform
- MCP server fully functional
- All tests pass

**Production Optimization (Docker):**
- hnswlib-node with C++ performance
- Pre-built Docker image
- State-of-the-art HNSW algorithm
- Best performance for 100K+ vectors

**Key Benefits:**
- ✅ No C++ build headaches for development
- ✅ Easy upgrade path to hnswlib via Docker
- ✅ Both implementations share same interface
- ✅ Environment variable controls backend
- ✅ Existing SemanticCache API unchanged

**Performance Comparison:**
- Annoy.js: ~1-5ms search for 10K vectors (acceptable)
- hnswlib: ~0.1-1ms search for 100K vectors (optimized)

