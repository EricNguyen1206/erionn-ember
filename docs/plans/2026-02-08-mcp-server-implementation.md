# MCP Server Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Transform the HTTP-based semantic cache into an MCP (Model Context Protocol) server using stdio transport, removing Fastify/Groq dependencies while maintaining the core caching functionality.

**Architecture:** The MCP server uses stdio transport with JSON-RPC 2.0 messages. It exposes 5 tools for AI completion with caching, cache management, and embedding generation. The existing SemanticCache and supporting libraries are preserved, while HTTP routes and GroqService are removed.

**Tech Stack:** Bun runtime, @modelcontextprotocol/sdk, zod validation, existing hnswlib-node/lz4js/xxhash-addon libraries

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
- Delete: `benchmark/` directory
- Delete: `src/server.js`
- Delete: `src/routes/chat.js`
- Delete: `src/services/groq.service.js`
- Delete: `src/services/chat.service.js`
- Delete: `src/services/index.js`

**Step 1: Install MCP SDK dependency**

Run:
```bash
bun add @modelcontextprotocol/sdk zod
```

Expected: Packages installed successfully

**Step 2: Update package.json scripts**

Modify `package.json`:
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
    "test:watch": "bun test --watch tests/"
  },
  "dependencies": {
    "@modelcontextprotocol/sdk": "^1.0.0",
    "hnswlib-node": "^2.0.0",
    "lz4js": "^0.2.0",
    "xxhash-addon": "^2.0.0",
    "zod": "^3.22.0"
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

**Step 3: Remove obsolete files and benchmark folder**

Run:
```bash
rm -rf benchmark/
rm -f src/server.js
rm -f src/routes/chat.js
rm -rf src/routes/
rm -f src/services/groq.service.js
rm -f src/services/chat.service.js
rm -f src/services/index.js
rm -rf src/services/
```

Expected: Files deleted successfully

**Step 4: Commit**

Run:
```bash
git add package.json
rm -rf benchmark src/server.js src/routes src/services
bun install
git add bun.lock
git commit -m "chore: remove HTTP API, Fastify, and Groq dependencies

- Delete benchmark/ folder (HTTP load tests not applicable)
- Remove Fastify server and routes
- Remove GroqService and ChatService
- Add MCP SDK dependency
- Update package.json for MCP server"
```

---

### Task 2: Create Embedding Service

**Files:**
- Create: `src/services/embedding-service.js`
- Test: `tests/services/embedding-service.test.js`

**Step 1: Write failing test**

Create `tests/services/embedding-service.test.js`:
```javascript
import { describe, test, expect, beforeEach } from 'bun:test';
import EmbeddingService from '../../src/services/embedding-service.js';

describe('EmbeddingService', () => {
  let service;

  beforeEach(() => {
    service = new EmbeddingService({
      provider: 'mock',
      apiKey: 'test-key'
    });
  });

  test('should generate embedding for text', async () => {
    const result = await service.generate('Hello world');
    
    expect(result).toBeDefined();
    expect(Array.isArray(result.embedding)).toBe(true);
    expect(result.embedding.length).toBe(1536);
  });

  test('should return null when provider not configured', async () => {
    const unconfiguredService = new EmbeddingService({
      provider: 'openai',
      apiKey: null
    });
    
    const result = await unconfiguredService.generate('Hello');
    expect(result).toBeNull();
  });

  test('should check if service is configured', () => {
    expect(service.isConfigured()).toBe(true);
    
    const unconfigured = new EmbeddingService({
      provider: 'openai',
      apiKey: null
    });
    expect(unconfigured.isConfigured()).toBe(false);
  });
});
```

**Step 2: Run test to verify it fails**

Run:
```bash
bun test tests/services/embedding-service.test.js
```

Expected: FAIL - "Cannot find module"

**Step 3: Create services directory and implement EmbeddingService**

Create directory:
```bash
mkdir -p src/services
```

Create `src/services/embedding-service.js`:
```javascript
/**
 * EmbeddingService - Generates vector embeddings for text
 * Supports multiple providers: mock (for testing), openai
 */
export class EmbeddingService {
  constructor(options = {}) {
    this.provider = options.provider || 'mock';
    this.apiKey = options.apiKey || null;
    this.model = options.model || 'text-embedding-3-small';
    this.dimension = options.dimension || 1536;
  }

  /**
   * Generate embedding for text
   * @param {string} text - Text to embed
   * @returns {Promise<{embedding: number[], model: string}|null>} Embedding result or null
   */
  async generate(text) {
    if (!this.isConfigured()) {
      return null;
    }

    if (this.provider === 'mock') {
      // Generate deterministic mock embedding based on text hash
      return this._generateMockEmbedding(text);
    }

    if (this.provider === 'openai') {
      return this._generateOpenAIEmbedding(text);
    }

    return null;
  }

  /**
   * Check if service is properly configured
   * @returns {boolean}
   */
  isConfigured() {
    if (this.provider === 'mock') {
      return true; // Mock always works
    }
    return Boolean(this.apiKey);
  }

  /**
   * Generate deterministic mock embedding
   * @private
   */
  _generateMockEmbedding(text) {
    // Create a deterministic embedding based on text content
    // This ensures same text = same embedding for testing
    const embedding = new Array(this.dimension);
    let seed = 0;
    
    for (let i = 0; i < text.length; i++) {
      seed += text.charCodeAt(i);
    }
    
    for (let i = 0; i < this.dimension; i++) {
      // Simple pseudo-random based on seed
      const x = Math.sin(seed + i * 12.9898) * 43758.5453;
      embedding[i] = x - Math.floor(x);
    }
    
    // Normalize to unit vector
    const magnitude = Math.sqrt(embedding.reduce((sum, val) => sum + val * val, 0));
    const normalized = embedding.map(val => val / magnitude);
    
    return {
      embedding: normalized,
      model: 'mock-embedding-model'
    };
  }

  /**
   * Generate embedding using OpenAI API
   * @private
   */
  async _generateOpenAIEmbedding(text) {
    try {
      const response = await fetch('https://api.openai.com/v1/embeddings', {
        method: 'POST',
        headers: {
          'Authorization': `Bearer ${this.apiKey}`,
          'Content-Type': 'application/json'
        },
        body: JSON.stringify({
          model: this.model,
          input: text
        })
      });

      if (!response.ok) {
        const error = await response.text();
        console.error('Embedding API error:', error);
        return null;
      }

      const data = await response.json();
      
      return {
        embedding: data.data[0].embedding,
        model: this.model
      };
    } catch (err) {
      console.error('Failed to generate embedding:', err.message);
      return null;
    }
  }
}

export default EmbeddingService;
```

**Step 4: Run test to verify it passes**

Run:
```bash
bun test tests/services/embedding-service.test.js
```

Expected: PASS - All tests pass

**Step 5: Commit**

Run:
```bash
git add src/services/embedding-service.js tests/services/embedding-service.test.js
git commit -m "feat: add EmbeddingService for vector generation

- Support mock provider (deterministic embeddings for testing)
- Support OpenAI provider (text-embedding-3-small)
- Add comprehensive unit tests"
```

---

### Task 3: Create MCP Tool Handlers

**Files:**
- Create: `src/tools/` directory
- Create: `src/tools/ai-complete.js`
- Create: `src/tools/cache-check.js`
- Create: `src/tools/cache-store.js`
- Create: `src/tools/cache-stats.js`
- Create: `src/tools/generate-embedding.js`

**Step 1: Create tools directory and ai-complete handler**

Create directory:
```bash
mkdir -p src/tools
```

Create `src/tools/ai-complete.js`:
```javascript
import { z } from 'zod';

const aiCompleteSchema = z.object({
  prompt: z.string().min(1),
  embedding: z.array(z.number()).optional(),
  metadata: z.record(z.any()).optional(),
  similarityThreshold: z.number().min(0).max(1).optional()
});

/**
 * AI Complete tool handler
 * Checks cache for similar prompts and returns cached response if found
 * @param {object} params - Tool parameters
 * @param {SemanticCache} cache - Cache instance
 * @returns {object} Tool result
 */
export async function handleAiComplete(params, cache) {
  const validated = aiCompleteSchema.parse(params);
  const { prompt, embedding, metadata, similarityThreshold } = validated;

  // Check cache
  const cacheResult = await cache.get(prompt, embedding, {
    minSimilarity: similarityThreshold
  });

  if (cacheResult) {
    // Cache hit
    return {
      content: [{
        type: 'text',
        text: JSON.stringify({
          cached: true,
          response: cacheResult.response,
          similarity: cacheResult.similarity,
          isExactMatch: cacheResult.isExactMatch,
          cachedAt: cacheResult.cachedAt
        }, null, 2)
      }]
    };
  }

  // Cache miss - client needs to call AI provider
  return {
    content: [{
      type: 'text',
      text: JSON.stringify({
        cached: false,
        message: 'Cache miss. Please call your AI provider and then use cache_store to save the response.'
      }, null, 2)
    }]
  };
}

export default handleAiComplete;
```

**Step 2: Create cache-check handler**

Create `src/tools/cache-check.js`:
```javascript
import { z } from 'zod';

const cacheCheckSchema = z.object({
  prompt: z.string().min(1),
  embedding: z.array(z.number()).optional(),
  similarityThreshold: z.number().min(0).max(1).optional()
});

/**
 * Cache Check tool handler
 * Check if a prompt exists in cache without storing anything
 * @param {object} params - Tool parameters
 * @param {SemanticCache} cache - Cache instance
 * @returns {object} Tool result
 */
export async function handleCacheCheck(params, cache) {
  const validated = cacheCheckSchema.parse(params);
  const { prompt, embedding, similarityThreshold } = validated;

  const cacheResult = await cache.get(prompt, embedding, {
    minSimilarity: similarityThreshold
  });

  if (cacheResult) {
    return {
      content: [{
        type: 'text',
        text: JSON.stringify({
          found: true,
          response: cacheResult.response,
          similarity: cacheResult.similarity,
          isExactMatch: cacheResult.isExactMatch,
          cachedAt: cacheResult.cachedAt
        }, null, 2)
      }]
    };
  }

  return {
    content: [{
      type: 'text',
      text: JSON.stringify({
        found: false,
        message: 'No matching entry found in cache'
      }, null, 2)
    }]
  };
}

export default handleCacheCheck;
```

**Step 3: Create cache-store handler**

Create `src/tools/cache-store.js`:
```javascript
import { z } from 'zod';

const cacheStoreSchema = z.object({
  prompt: z.string().min(1),
  response: z.string().min(1),
  embedding: z.array(z.number()).optional(),
  metadata: z.record(z.any()).optional(),
  ttl: z.number().positive().optional()
});

/**
 * Cache Store tool handler
 * Store a prompt/response pair in the cache
 * @param {object} params - Tool parameters
 * @param {SemanticCache} cache - Cache instance
 * @param {EmbeddingService} embeddingService - Embedding service instance
 * @returns {object} Tool result
 */
export async function handleCacheStore(params, cache, embeddingService) {
  const validated = cacheStoreSchema.parse(params);
  const { prompt, response, embedding, metadata, ttl } = validated;

  // Use provided embedding or generate one
  let vector = embedding;
  if (!vector && embeddingService.isConfigured()) {
    const embeddingResult = await embeddingService.generate(prompt);
    if (embeddingResult) {
      vector = embeddingResult.embedding;
    }
  }

  // Store in cache (exact-match only if no embedding)
  await cache.set(prompt, response, vector || new Array(1536).fill(0), {
    ttl,
    metadata
  });

  return {
    content: [{
      type: 'text',
      text: JSON.stringify({
        success: true,
        message: 'Response stored in cache',
        hasEmbedding: Boolean(vector)
      }, null, 2)
    }]
  };
}

export default handleCacheStore;
```

**Step 4: Create cache-stats handler**

Create `src/tools/cache-stats.js`:
```javascript
/**
 * Cache Stats tool handler
 * Get cache statistics and savings metrics
 * @param {object} params - Tool parameters (unused)
 * @param {SemanticCache} cache - Cache instance
 * @returns {object} Tool result
 */
export async function handleCacheStats(params, cache) {
  const stats = cache.getStats();

  return {
    content: [{
      type: 'text',
      text: JSON.stringify({
        totalEntries: stats.totalEntries,
        memoryUsage: stats.memoryUsage,
        compressionRatio: stats.compressionRatio,
        cacheHits: stats.cacheHits,
        cacheMisses: stats.cacheMisses,
        hitRate: stats.hitRate,
        totalQueries: stats.totalQueries,
        savedTokens: stats.savedTokens,
        savedUsd: stats.savedUsd
      }, null, 2)
    }]
  };
}

export default handleCacheStats;
```

**Step 5: Create generate-embedding handler**

Create `src/tools/generate-embedding.js`:
```javascript
import { z } from 'zod';

const generateEmbeddingSchema = z.object({
  text: z.string().min(1),
  model: z.string().optional()
});

/**
 * Generate Embedding tool handler
 * Generate embedding vector for text
 * @param {object} params - Tool parameters
 * @param {EmbeddingService} embeddingService - Embedding service instance
 * @returns {object} Tool result
 */
export async function handleGenerateEmbedding(params, embeddingService) {
  const validated = generateEmbeddingSchema.parse(params);
  const { text, model } = validated;

  if (!embeddingService.isConfigured()) {
    return {
      content: [{
        type: 'text',
        text: JSON.stringify({
          error: 'Embedding service not configured',
          message: 'Please set OPENAI_API_KEY environment variable or use mock provider'
        }, null, 2)
      }],
      isError: true
    };
  }

  const result = await embeddingService.generate(text, model);

  if (!result) {
    return {
      content: [{
        type: 'text',
        text: JSON.stringify({
          error: 'Failed to generate embedding'
        }, null, 2)
      }],
      isError: true
    };
  }

  return {
    content: [{
      type: 'text',
      text: JSON.stringify({
        embedding: result.embedding,
        model: result.model,
        dimension: result.embedding.length
      }, null, 2)
    }]
  };
}

export default handleGenerateEmbedding;
```

**Step 6: Commit**

Run:
```bash
git add src/tools/
git commit -m "feat: add MCP tool handlers

- ai_complete: Check cache and return result or cache miss
- cache_check: Check cache without storing
- cache_store: Store prompt/response with optional embedding
- cache_stats: Get cache metrics and savings
- generate_embedding: Generate vector embeddings"
```

---

### Task 4: Create MCP Server Entry Point

**Files:**
- Create: `src/mcp-server.js`
- Test: `tests/mcp-server.test.js`

**Step 1: Write failing test**

Create `tests/mcp-server.test.js`:
```javascript
import { describe, test, expect, beforeAll, afterAll } from 'bun:test';
import { spawn } from 'child_process';
import { fileURLToPath } from 'url';
import { dirname, join } from 'path';

const __dirname = dirname(fileURLToPath(import.meta.url));
const serverPath = join(__dirname, '..', 'src', 'mcp-server.js');

describe('MCP Server', () => {
  let serverProcess;
  let messageId = 0;

  beforeAll(() => {
    serverProcess = spawn('bun', [serverPath], {
      stdio: ['pipe', 'pipe', 'pipe'],
      env: {
        ...process.env,
        NODE_ENV: 'test',
        EMBEDDING_PROVIDER: 'mock'
      }
    });
  });

  afterAll(() => {
    if (serverProcess) {
      serverProcess.kill();
    }
  });

  function sendRequest(method, params = {}) {
    return new Promise((resolve, reject) => {
      const id = ++messageId;
      const request = {
        jsonrpc: '2.0',
        id,
        method,
        params
      };

      let response = '';
      
      const onData = (data) => {
        response += data.toString();
        const lines = response.split('\n');
        
        for (const line of lines) {
          if (line.trim()) {
            try {
              const parsed = JSON.parse(line);
              if (parsed.id === id) {
                serverProcess.stdout.off('data', onData);
                resolve(parsed);
                return;
              }
            } catch (e) {
              // Not valid JSON yet, continue reading
            }
          }
        }
      };

      serverProcess.stdout.on('data', onData);
      serverProcess.stdin.write(JSON.stringify(request) + '\n');

      // Timeout after 5 seconds
      setTimeout(() => {
        serverProcess.stdout.off('data', onData);
        reject(new Error('Request timeout'));
      }, 5000);
    });
  }

  test('should handle initialize request', async () => {
    const result = await sendRequest('initialize', {
      protocolVersion: '2024-11-05',
      capabilities: {},
      clientInfo: { name: 'test-client', version: '1.0.0' }
    });

    expect(result.error).toBeUndefined();
    expect(result.result).toBeDefined();
    expect(result.result.protocolVersion).toBe('2024-11-05');
  });

  test('should list available tools', async () => {
    const result = await sendRequest('tools/list', {});

    expect(result.error).toBeUndefined();
    expect(result.result).toBeDefined();
    expect(result.result.tools).toBeDefined();
    expect(result.result.tools.length).toBeGreaterThan(0);
  });
});
```

**Step 2: Run test to verify it fails**

Run:
```bash
bun test tests/mcp-server.test.js
```

Expected: FAIL - "Cannot find module" or timeout

**Step 3: Create MCP server entry point**

Create `src/mcp-server.js`:
```javascript
import { Server } from '@modelcontextprotocol/sdk/server/index.js';
import { StdioServerTransport } from '@modelcontextprotocol/sdk/server/stdio.js';
import {
  CallToolRequestSchema,
  ListToolsRequestSchema
} from '@modelcontextprotocol/sdk/types.js';
import SemanticCache from './lib/semantic-cache.js';
import EmbeddingService from './services/embedding-service.js';
import handleAiComplete from './tools/ai-complete.js';
import handleCacheCheck from './tools/cache-check.js';
import handleCacheStore from './tools/cache-store.js';
import handleCacheStats from './tools/cache-stats.js';
import handleGenerateEmbedding from './tools/generate-embedding.js';

// Configuration from environment
const config = {
  similarityThreshold: parseFloat(process.env.CACHE_SIMILARITY_THRESHOLD) || 0.85,
  maxElements: parseInt(process.env.CACHE_MAX_ELEMENTS) || 100000,
  defaultTTL: parseInt(process.env.CACHE_DEFAULT_TTL) || 3600,
  embeddingProvider: process.env.EMBEDDING_PROVIDER || 'mock',
  openaiApiKey: process.env.OPENAI_API_KEY || null
};

// Initialize services
const cache = new SemanticCache({
  dim: 1536,
  maxElements: config.maxElements,
  similarityThreshold: config.similarityThreshold,
  defaultTTL: config.defaultTTL
});

const embeddingService = new EmbeddingService({
  provider: config.embeddingProvider,
  apiKey: config.openaiApiKey,
  dimension: 1536
});

// Log startup info to stderr (not stdout - that's for MCP messages)
console.error('🚀 Starting Erion Ember MCP Server');
console.error(`   Embedding provider: ${config.embeddingProvider}`);
console.error(`   Similarity threshold: ${config.similarityThreshold}`);

// Create MCP server
const server = new Server(
  {
    name: 'erion-ember-semantic-cache',
    version: '1.0.0'
  },
  {
    capabilities: {
      tools: {}
    }
  }
);

// List available tools
server.setRequestHandler(ListToolsRequestSchema, async () => {
  return {
    tools: [
      {
        name: 'ai_complete',
        description: 'Complete a prompt using AI with semantic caching. Checks cache first, returns cached response if found, or indicates cache miss.',
        inputSchema: {
          type: 'object',
          properties: {
            prompt: {
              type: 'string',
              description: 'The prompt to complete'
            },
            embedding: {
              type: 'array',
              items: { type: 'number' },
              description: 'Optional pre-computed embedding vector for semantic search'
            },
            metadata: {
              type: 'object',
              description: 'Optional metadata to store with the cached entry'
            },
            similarityThreshold: {
              type: 'number',
              minimum: 0,
              maximum: 1,
              description: 'Override default similarity threshold (0-1)'
            }
          },
          required: ['prompt']
        }
      },
      {
        name: 'cache_check',
        description: 'Check if a prompt exists in cache without storing anything. Useful for pre-flight checks.',
        inputSchema: {
          type: 'object',
          properties: {
            prompt: {
              type: 'string',
              description: 'The prompt to check'
            },
            embedding: {
              type: 'array',
              items: { type: 'number' },
              description: 'Optional pre-computed embedding vector'
            },
            similarityThreshold: {
              type: 'number',
              minimum: 0,
              maximum: 1,
              description: 'Override default similarity threshold (0-1)'
            }
          },
          required: ['prompt']
        }
      },
      {
        name: 'cache_store',
        description: 'Store a prompt and its AI response in the semantic cache. Optionally generates embedding if not provided.',
        inputSchema: {
          type: 'object',
          properties: {
            prompt: {
              type: 'string',
              description: 'The prompt that was sent to the AI'
            },
            response: {
              type: 'string',
              description: 'The AI response to cache'
            },
            embedding: {
              type: 'array',
              items: { type: 'number' },
              description: 'Optional pre-computed embedding vector'
            },
            metadata: {
              type: 'object',
              description: 'Optional metadata to store'
            },
            ttl: {
              type: 'number',
              description: 'Time-to-live in seconds'
            }
          },
          required: ['prompt', 'response']
        }
      },
      {
        name: 'cache_stats',
        description: 'Get cache statistics including hit rate, memory usage, and cost savings',
        inputSchema: {
          type: 'object',
          properties: {}
        }
      },
      {
        name: 'generate_embedding',
        description: 'Generate embedding vector for text. Useful when you want to manage embeddings yourself.',
        inputSchema: {
          type: 'object',
          properties: {
            text: {
              type: 'string',
              description: 'Text to generate embedding for'
            },
            model: {
              type: 'string',
              description: 'Optional model override'
            }
          },
          required: ['text']
        }
      }
    ]
  };
});

// Handle tool calls
server.setRequestHandler(CallToolRequestSchema, async (request) => {
  const { name, arguments: args } = request.params;

  try {
    switch (name) {
      case 'ai_complete':
        return await handleAiComplete(args, cache);
      
      case 'cache_check':
        return await handleCacheCheck(args, cache);
      
      case 'cache_store':
        return await handleCacheStore(args, cache, embeddingService);
      
      case 'cache_stats':
        return await handleCacheStats(args, cache);
      
      case 'generate_embedding':
        return await handleGenerateEmbedding(args, embeddingService);
      
      default:
        throw new Error(`Unknown tool: ${name}`);
    }
  } catch (error) {
    console.error(`Error handling tool ${name}:`, error.message);
    
    return {
      content: [{
        type: 'text',
        text: JSON.stringify({
          error: error.message,
          tool: name
        }, null, 2)
      }],
      isError: true
    };
  }
});

// Start server with stdio transport
async function main() {
  const transport = new StdioServerTransport();
  
  // Graceful shutdown
  process.on('SIGINT', async () => {
    console.error('\n🔄 Shutting down gracefully...');
    cache.destroy();
    await server.close();
    process.exit(0);
  });

  process.on('SIGTERM', async () => {
    console.error('\n🔄 Shutting down gracefully...');
    cache.destroy();
    await server.close();
    process.exit(0);
  });

  await server.connect(transport);
  console.error('✅ MCP Server ready');
}

main().catch((error) => {
  console.error('Fatal error:', error);
  process.exit(1);
});
```

**Step 4: Run test to verify it passes**

Run:
```bash
bun test tests/mcp-server.test.js
```

Expected: PASS - All tests pass

**Step 5: Commit**

Run:
```bash
git add src/mcp-server.js tests/mcp-server.test.js
git commit -m "feat: create MCP server entry point with stdio transport

- Initialize MCP server with @modelcontextprotocol/sdk
- Register all 5 tools with proper schemas
- Wire up SemanticCache and EmbeddingService
- Add graceful shutdown handlers"
```

---

### Task 5: Create MCP Integration Tests

**Files:**
- Create: `tests/mcp/` directory
- Create: `tests/mcp/integration.test.js`

**Step 1: Create integration test file**

Create directory:
```bash
mkdir -p tests/mcp
```

Create `tests/mcp/integration.test.js`:
```javascript
import { describe, test, expect, beforeAll, afterAll } from 'bun:test';
import { spawn } from 'child_process';
import { fileURLToPath } from 'url';
import { dirname, join } from 'path';

const __dirname = dirname(fileURLToPath(import.meta.url));
const serverPath = join(__dirname, '..', '..', 'src', 'mcp-server.js');

class MCPTestClient {
  constructor(process) {
    this.process = process;
    this.messageId = 0;
  }

  async request(method, params = {}) {
    return new Promise((resolve, reject) => {
      const id = ++this.messageId;
      const request = {
        jsonrpc: '2.0',
        id,
        method,
        params
      };

      let buffer = '';
      
      const onData = (data) => {
        buffer += data.toString();
        const lines = buffer.split('\n');
        
        for (const line of lines) {
          if (!line.trim()) continue;
          
          try {
            const parsed = JSON.parse(line);
            if (parsed.id === id) {
              this.process.stdout.off('data', onData);
              resolve(parsed);
              return;
            }
          } catch (e) {
            // Not valid JSON, continue reading
          }
        }
      };

      this.process.stdout.on('data', onData);
      this.process.stdin.write(JSON.stringify(request) + '\n');

      setTimeout(() => {
        this.process.stdout.off('data', onData);
        reject(new Error('Request timeout'));
      }, 5000);
    });
  }

  async callTool(name, args) {
    return this.request('tools/call', { name, arguments: args });
  }
}

describe('MCP Integration Tests', () => {
  let serverProcess;
  let client;

  beforeAll(() => {
    serverProcess = spawn('bun', [serverPath], {
      stdio: ['pipe', 'pipe', 'pipe'],
      env: {
        ...process.env,
        NODE_ENV: 'test',
        EMBEDDING_PROVIDER: 'mock'
      }
    });
    client = new MCPTestClient(serverProcess);
  });

  afterAll(() => {
    if (serverProcess) {
      serverProcess.kill();
    }
  });

  test('full cache workflow: miss -> store -> hit', async () => {
    const prompt = 'What is semantic caching?';
    const response = 'Semantic caching stores AI responses based on meaning, not just exact text matches.';

    // Step 1: Check cache - should miss
    const checkResult = await client.callTool('ai_complete', { prompt });
    expect(checkResult.result.content[0].text).toContain('cached":false');

    // Step 2: Store response
    const storeResult = await client.callTool('cache_store', { 
      prompt, 
      response 
    });
    expect(storeResult.result.content[0].text).toContain('success":true');

    // Step 3: Check cache again - should hit
    const hitResult = await client.callTool('ai_complete', { prompt });
    expect(hitResult.result.content[0].text).toContain('cached":true');
    expect(hitResult.result.content[0].text).toContain(response);
  });

  test('cache_check tool works independently', async () => {
    const result = await client.callTool('cache_check', { 
      prompt: 'Non-existent prompt' 
    });
    
    expect(result.result.content[0].text).toContain('found":false');
  });

  test('cache_stats returns metrics', async () => {
    const result = await client.callTool('cache_stats', {});
    const stats = JSON.parse(result.result.content[0].text);
    
    expect(stats).toHaveProperty('totalEntries');
    expect(stats).toHaveProperty('hitRate');
    expect(stats).toHaveProperty('cacheHits');
    expect(stats).toHaveProperty('cacheMisses');
  });

  test('generate_embedding creates vector', async () => {
    const result = await client.callTool('generate_embedding', { 
      text: 'Test text' 
    });
    const embedding = JSON.parse(result.result.content[0].text);
    
    expect(embedding).toHaveProperty('embedding');
    expect(Array.isArray(embedding.embedding)).toBe(true);
    expect(embedding.embedding.length).toBe(1536);
  });

  test('handles invalid tool gracefully', async () => {
    const result = await client.callTool('nonexistent_tool', {});
    
    expect(result.result.isError).toBe(true);
    expect(result.result.content[0].text).toContain('Unknown tool');
  });

  test('handles invalid parameters', async () => {
    const result = await client.callTool('cache_store', { 
      // Missing required 'response' parameter
      prompt: 'test' 
    });
    
    expect(result.result.isError).toBe(true);
  });
});
```

**Step 2: Run integration tests**

Run:
```bash
bun test tests/mcp/integration.test.js
```

Expected: PASS - All integration tests pass

**Step 3: Commit**

Run:
```bash
git add tests/mcp/
git commit -m "test: add MCP integration tests

- Test full workflow: miss -> store -> hit
- Test each tool independently
- Test error handling for invalid tools/params
- Add MCPTestClient helper class"
```

---

### Task 6: Update README Documentation

**Files:**
- Modify: `README.md`

**Step 1: Update README for MCP server**

Read current README and replace with MCP-focused documentation:

Replace `README.md` content with:
```markdown
# 🚀 Erion Ember

LLM Semantic Cache MCP Server - Production-ready semantic caching for AI coding assistants via the Model Context Protocol.

## Overview

Erion Ember provides an MCP server that caches LLM responses using semantic similarity matching. It integrates with AI coding assistants like Claude Code, Opencode, and Codex to reduce API costs and latency.

## Features

- ✅ **MCP Protocol**: Standardized tool interface for AI assistants
- ✅ **Semantic Caching**: Intelligent cache with vector similarity matching
- ✅ **Multi-Provider**: Works with any AI provider (Claude, OpenAI, Groq, etc.)
- ✅ **Embedding Generation**: Built-in embedding service (OpenAI or mock)
- ✅ **Cost Tracking**: Monitor token savings and cost reductions
- ✅ **Bun Runtime**: Blazing fast JavaScript runtime

## Installation

```bash
# Clone repository
git clone https://github.com/yourusername/erion-ember.git
cd erion-ember

# Install dependencies
bun install
```

## Configuration

Create `.env` file:

```bash
# Required: Choose embedding provider (mock or openai)
EMBEDDING_PROVIDER=mock

# Optional: OpenAI API key (if using openai provider)
OPENAI_API_KEY=sk-...

# Optional: Cache configuration
CACHE_SIMILARITY_THRESHOLD=0.85
CACHE_MAX_ELEMENTS=100000
CACHE_DEFAULT_TTL=3600
```

## Usage with MCP Clients

### Claude Code

Add to Claude Code configuration:

```json
{
  "mcpServers": {
    "erion-ember": {
      "command": "bun",
      "args": ["run", "/path/to/erion-ember/src/mcp-server.js"],
      "env": {
        "EMBEDDING_PROVIDER": "mock"
      }
    }
  }
}
```

### Opencode

Add to `.opencode/config.json`:

```json
{
  "mcpServers": [
    {
      "name": "erion-ember",
      "command": "bun run /path/to/erion-ember/src/mcp-server.js",
      "env": {
        "EMBEDDING_PROVIDER": "mock"
      }
    }
  ]
}
```

## Available Tools

### `ai_complete`

Check cache for a prompt and return cached response or indicate cache miss.

**Parameters:**
- `prompt` (string, required): The prompt to complete
- `embedding` (number[], optional): Pre-computed embedding vector
- `metadata` (object, optional): Additional metadata to store
- `similarityThreshold` (number, optional): Override similarity threshold (0-1)

**Response (cache hit):**
```json
{
  "cached": true,
  "response": "Cached response text...",
  "similarity": 0.95,
  "isExactMatch": false,
  "cachedAt": "2026-02-08T10:30:00.000Z"
}
```

**Response (cache miss):**
```json
{
  "cached": false,
  "message": "Cache miss. Please call your AI provider..."
}
```

### `cache_store`

Store a prompt/response pair in the cache.

**Parameters:**
- `prompt` (string, required): The prompt to cache
- `response` (string, required): The AI response
- `embedding` (number[], optional): Pre-computed embedding
- `metadata` (object, optional): Additional metadata
- `ttl` (number, optional): Time-to-live in seconds

### `cache_check`

Check if a prompt exists in cache without storing.

**Parameters:**
- `prompt` (string, required): The prompt to check
- `embedding` (number[], optional): Pre-computed embedding
- `similarityThreshold` (number, optional): Override similarity threshold

### `generate_embedding`

Generate embedding vector for text.

**Parameters:**
- `text` (string, required): Text to embed
- `model` (string, optional): Embedding model to use

**Response:**
```json
{
  "embedding": [0.1, 0.2, ...],
  "model": "mock-embedding-model",
  "dimension": 1536
}
```

### `cache_stats`

Get cache statistics.

**Response:**
```json
{
  "totalEntries": 100,
  "memoryUsage": { "vectors": 153600, "metadata": 10240 },
  "compressionRatio": "0.45",
  "cacheHits": 250,
  "cacheMisses": 50,
  "hitRate": "0.8333",
  "totalQueries": 300,
  "savedTokens": 15000,
  "savedUsd": 0.45
}
```

## Workflow Example

```javascript
// 1. Check cache first
const result = await mcpClient.callTool('ai_complete', {
  prompt: 'Explain quantum computing'
});

if (result.cached) {
  // Use cached response
  return result.response;
}

// 2. Cache miss - call your AI provider
const aiResponse = await callClaudeAPI('Explain quantum computing');

// 3. Store in cache for future use
await mcpClient.callTool('cache_store', {
  prompt: 'Explain quantum computing',
  response: aiResponse
});

return aiResponse;
```

## Development

```bash
# Run in development mode
bun run dev

# Run tests
bun test

# Run specific test file
bun test tests/mcp/integration.test.js
```

## Testing

The project includes comprehensive tests:

- **Unit tests**: Individual components (SemanticCache, EmbeddingService)
- **Integration tests**: Full MCP protocol workflow
- **MCP compliance**: Protocol-level message handling

```bash
# Run all tests
bun test

# Run with coverage
bun test --coverage
```

## Project Structure

```
erion-ember/
├── src/
│   ├── mcp-server.js          # MCP server entry point
│   ├── lib/                   # Core caching logic
│   │   ├── semantic-cache.js
│   │   ├── hnsw-index.js
│   │   ├── quantizer.js
│   │   ├── compressor.js
│   │   ├── normalizer.js
│   │   └── metadata-store.js
│   ├── services/
│   │   └── embedding-service.js
│   └── tools/                 # MCP tool handlers
│       ├── ai-complete.js
│       ├── cache-check.js
│       ├── cache-store.js
│       ├── cache-stats.js
│       └── generate-embedding.js
├── tests/
│   ├── lib/                   # Core library tests
│   ├── services/              # Service tests
│   ├── mcp/                   # MCP integration tests
│   └── mcp-server.test.js     # Server protocol tests
└── package.json
```

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `EMBEDDING_PROVIDER` | Embedding provider: `mock` or `openai` | `mock` |
| `OPENAI_API_KEY` | OpenAI API key (required if provider=openai) | - |
| `CACHE_SIMILARITY_THRESHOLD` | Minimum similarity for cache hits | `0.85` |
| `CACHE_MAX_ELEMENTS` | Maximum cache entries | `100000` |
| `CACHE_DEFAULT_TTL` | Default TTL in seconds | `3600` |
| `NODE_ENV` | Environment mode | `development` |

## License

MIT License - see LICENSE file for details.
```

**Step 2: Commit**

Run:
```bash
git add README.md
git commit -m "docs: update README for MCP server

- Replace HTTP API documentation with MCP protocol
- Add configuration examples for Claude Code and Opencode
- Document all 5 available tools
- Add workflow examples and environment variables"
```

---

### Task 7: Final Verification and Cleanup

**Step 1: Run all tests**

Run:
```bash
bun test
```

Expected: All tests pass

**Step 2: Verify MCP server starts correctly**

Run:
```bash
bun run start
```

Expected: Server starts, outputs to stderr:
```
🚀 Starting Erion Ember MCP Server
   Embedding provider: mock
   Similarity threshold: 0.85
✅ MCP Server ready
```

Press Ctrl+C to stop.

**Step 3: Final commit**

Run:
```bash
git add .
git commit -m "refactor: transform HTTP API to MCP server

- Remove Fastify, GroqService, and HTTP routes
- Add MCP stdio transport with JSON-RPC
- Create 5 MCP tools for caching workflow
- Add EmbeddingService with mock and OpenAI providers
- Update all tests for MCP protocol
- Delete benchmark/ folder (not applicable to stdio)
- Update documentation for MCP usage"
```

---

## Summary

This implementation plan transforms the HTTP-based semantic cache into a pure MCP server:

1. **Removed**: Fastify, GroqService, HTTP routes, benchmark folder
2. **Added**: MCP SDK, EmbeddingService, 5 tool handlers, stdio transport
3. **Preserved**: SemanticCache and all supporting libraries
4. **Tests**: Updated for MCP protocol, added integration tests
5. **Docs**: Rewrote README for MCP clients

The server now works with any AI coding assistant that supports MCP, while maintaining the powerful semantic caching functionality.
