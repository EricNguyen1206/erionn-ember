import { Server } from '@modelcontextprotocol/sdk/server/index.js';
import { StdioServerTransport } from '@modelcontextprotocol/sdk/server/stdio.js';
import {
  CallToolRequestSchema,
  ListToolsRequestSchema,
} from '@modelcontextprotocol/sdk/types.js';
import { SemanticCache } from './lib/semantic-cache.js';
import { EmbeddingService } from './services/embedding-service.js';
import handleAiComplete from './tools/ai-complete.js';
import handleCacheCheck from './tools/cache-check.js';
import handleCacheStore from './tools/cache-store.js';
import handleCacheStats from './tools/cache-stats.js';
import handleGenerateEmbedding from './tools/generate-embedding.js';

interface ServerConfig {
  similarityThreshold: number;
  maxElements: number;
  defaultTTL: number;
}

const config: ServerConfig = {
  similarityThreshold: parseFloat(process.env.CACHE_SIMILARITY_THRESHOLD ?? '') || 0.85,
  maxElements: parseInt(process.env.CACHE_MAX_ELEMENTS ?? '', 10) || 100000,
  defaultTTL: parseInt(process.env.CACHE_DEFAULT_TTL ?? '', 10) || 3600,
};

const cache = new SemanticCache({
  dim: 384,
  maxElements: config.maxElements,
  similarityThreshold: config.similarityThreshold,
  defaultTTL: config.defaultTTL,
});

const embeddingService = new EmbeddingService();

console.error('Starting Erion Ember MCP Server');
console.error(`  Backend: ${process.env.VECTOR_INDEX_BACKEND || 'annoy'}`);
console.error(`  Threshold: ${config.similarityThreshold}`);

// Create MCP server
const server = new Server(
  {
    name: 'erion-ember-semantic-cache',
    version: '1.0.0',
  },
  {
    capabilities: {
      tools: {},
    },
  }
);

// List available tools
server.setRequestHandler(ListToolsRequestSchema, async () => {
  return {
    tools: [
      {
        name: 'ai_complete',
        description:
          'Complete a prompt using AI with semantic caching. Checks cache first, returns cached response if found, or indicates cache miss.',
        inputSchema: {
          type: 'object',
          properties: {
            prompt: {
              type: 'string',
              description: 'The prompt to complete',
            },
            embedding: {
              type: 'array',
              items: { type: 'number' },
              description: 'Optional pre-computed embedding vector for semantic search',
            },
            metadata: {
              type: 'object',
              description: 'Optional metadata to store with the cached entry',
            },
            similarityThreshold: {
              type: 'number',
              minimum: 0,
              maximum: 1,
              description: 'Override default similarity threshold (0-1)',
            },
          },
          required: ['prompt'],
        },
      },
      {
        name: 'cache_check',
        description:
          'Check if a prompt exists in cache without storing anything. Useful for pre-flight checks.',
        inputSchema: {
          type: 'object',
          properties: {
            prompt: {
              type: 'string',
              description: 'The prompt to check',
            },
            embedding: {
              type: 'array',
              items: { type: 'number' },
              description: 'Optional pre-computed embedding vector',
            },
            similarityThreshold: {
              type: 'number',
              minimum: 0,
              maximum: 1,
              description: 'Override default similarity threshold (0-1)',
            },
          },
          required: ['prompt'],
        },
      },
      {
        name: 'cache_store',
        description:
          'Store a prompt and its AI response in the semantic cache. Optionally generates embedding if not provided.',
        inputSchema: {
          type: 'object',
          properties: {
            prompt: {
              type: 'string',
              description: 'The prompt that was sent to the AI',
            },
            response: {
              type: 'string',
              description: 'The AI response to cache',
            },
            embedding: {
              type: 'array',
              items: { type: 'number' },
              description: 'Optional pre-computed embedding vector',
            },
            metadata: {
              type: 'object',
              description: 'Optional metadata to store',
            },
            ttl: {
              type: 'number',
              description: 'Time-to-live in seconds',
            },
          },
          required: ['prompt', 'response'],
        },
      },
      {
        name: 'cache_stats',
        description: 'Get cache statistics including hit rate, memory usage, and cost savings',
        inputSchema: {
          type: 'object',
          properties: {},
        },
      },
      {
        name: 'generate_embedding',
        description:
          'Generate embedding vector for text. Useful when you want to manage embeddings yourself.',
        inputSchema: {
          type: 'object',
          properties: {
            text: {
              type: 'string',
              description: 'Text to generate embedding for',
            },
            model: {
              type: 'string',
              description: 'Optional model override',
            },
          },
          required: ['text'],
        },
      },
    ],
  };
});

// Handle tool calls
server.setRequestHandler(CallToolRequestSchema, async (request) => {
  const { name, arguments: args } = request.params;
  const params = (args ?? {}) as Record<string, unknown>;

  try {
    switch (name) {
      case 'ai_complete':
        return await handleAiComplete(params, cache);

      case 'cache_check':
        return await handleCacheCheck(params, cache);

      case 'cache_store':
        return await handleCacheStore(params, cache, embeddingService);

      case 'cache_stats':
        return await handleCacheStats(params, cache);

      case 'generate_embedding':
        return await handleGenerateEmbedding(params, embeddingService);

      default:
        throw new Error(`Unknown tool: ${name}`);
    }
  } catch (error) {
    const err = error as Error;
    console.error(`Error handling tool ${name}:`, err.message);

    return {
      content: [
        {
          type: 'text' as const,
          text: JSON.stringify(
            {
              error: err.message,
              tool: name,
            },
            null,
            2
          ),
        },
      ],
      isError: true,
    };
  }
});

// Start server with stdio transport
async function main(): Promise<void> {
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

main().catch((error: Error) => {
  console.error('Fatal error:', error);
  process.exit(1);
});
