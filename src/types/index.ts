/**
 * Core type definitions for Erion Ember
 * LLM Semantic Cache MCP Server
 */

// ============================================================================
// Cache Configuration Types
// ============================================================================

export interface CacheConfig {
  /** Vector dimension (default: 1536) */
  dim?: number;
  /** Maximum number of cached entries (default: 100000) */
  maxElements?: number;
  /** Similarity threshold for cache hits 0-1 (default: 0.85) */
  similarityThreshold?: number;
  /** Memory limit string like '1gb' (default: '1gb') */
  memoryLimit?: string;
  /** Default TTL in seconds (default: 3600) */
  defaultTTL?: number;
}

export interface CacheOptions {
  /** Time-to-live in seconds */
  ttl?: number;
  /** Additional metadata to store */
  metadata?: Record<string, unknown>;
}

export interface QueryOptions {
  /** Minimum similarity threshold (overrides default) */
  minSimilarity?: number;
}

// ============================================================================
// Cache Data Types
// ============================================================================

export interface CacheMetadata {
  /** Unique entry ID */
  id: string;
  /** Vector index ID */
  vectorId: number;
  /** Hash of normalized prompt */
  promptHash: string;
  /** Normalized prompt text */
  normalizedPrompt: string;
  /** Compressed prompt data */
  compressedPrompt: Buffer;
  /** Compressed response data */
  compressedResponse: Buffer;
  /** Original prompt size in bytes */
  originalPromptSize: number;
  /** Original response size in bytes */
  originalResponseSize: number;
  /** Compressed prompt size in bytes */
  compressedPromptSize: number;
  /** Compressed response size in bytes */
  compressedResponseSize: number;
  /** @deprecated Legacy field — use compressedResponseSize instead */
  compressedSize?: number;
  /** Timestamp when entry was created */
  createdAt: number;
  /** Timestamp of last access */
  lastAccessed: number;
  /** Number of times accessed */
  accessCount: number;
  /** Expiration timestamp (if TTL set) */
  expiresAt?: number;
}

export interface CacheResult {
  /** Decompressed response text */
  response: string;
  /** Similarity score 0-1 */
  similarity: number;
  /** Whether this was an exact hash match */
  isExactMatch: boolean;
  /** When the entry was cached */
  cachedAt: Date;
  /** Full metadata object */
  metadata: CacheMetadata;
}

// ============================================================================
// Statistics Types
// ============================================================================

export interface MemoryUsage {
  /** Memory used by vectors in bytes */
  vectors: number;
  /** Memory used by metadata in bytes */
  metadata: number;
  /** Total memory usage in bytes */
  total: number;
}

export interface CacheStats {
  /** Total number of cached entries */
  totalEntries: number;
  /** Memory usage breakdown */
  memoryUsage: MemoryUsage;
  /** Compression ratio (compressed/original) */
  compressionRatio: string;
  /** Number of cache hits */
  cacheHits: number;
  /** Number of cache misses */
  cacheMisses: number;
  /** Hit rate as decimal string */
  hitRate: string;
  /** Total number of queries */
  totalQueries: number;
  /** Estimated tokens saved */
  savedTokens: number;
  /** Estimated USD saved */
  savedUsd: number;
}

export interface InternalStatistics {
  hits: number;
  misses: number;
  totalQueries: number;
  savedTokens: number;
  savedUsd: number;
}

// ============================================================================
// Vector Index Types
// ============================================================================

export type DistanceMetric = 'cosine' | 'l2' | 'ip';
export type VectorBackend = 'annoy' | 'hnsw' | 'turso' | 'qdrant';

export interface VectorIndexConfig {
  /** Vector dimension */
  dim: number;
  /** Maximum number of elements */
  maxElements: number;
  /** Distance metric (default: 'cosine') */
  space?: DistanceMetric;
  /** Backend type (default: 'annoy') */
  backend?: VectorBackend;
}

export interface SearchResult {
  /** Item ID */
  id: number;
  /** Distance from query (lower is more similar) */
  distance: number;
}

// ============================================================================
// Embedding Types
// ============================================================================

export interface EmbeddingResult {
  /** Embedding vector */
  embedding: number[];
  /** Model name used */
  model: string;
}

// ============================================================================
// MCP Tool Types
// ============================================================================

export interface ToolContent {
  type: 'text';
  text: string;
}

export interface ToolResult {
  content: ToolContent[];
  isError?: boolean;
  [key: string]: unknown;
}

export interface AiCompleteParams {
  prompt: string;
  embedding?: number[];
  metadata?: Record<string, unknown>;
  similarityThreshold?: number;
}

export interface CacheCheckParams {
  prompt: string;
  embedding?: number[];
  similarityThreshold?: number;
}

export interface CacheStoreParams {
  prompt: string;
  response: string;
  embedding?: number[];
  metadata?: Record<string, unknown>;
  ttl?: number;
}

export interface GenerateEmbeddingParams {
  text: string;
  model?: string;
}

// ============================================================================
// Metadata Store Types
// ============================================================================

export interface MetadataStoreConfig {
  /** Maximum number of entries (default: 100000) */
  maxSize?: number;
}

export interface MetadataStoreStats {
  totalEntries: number;
  totalCompressedSize: number;
  memoryLimit: number;
}

export interface LRUNode {
  id: string;
  prev: LRUNode | null;
  next: LRUNode | null;
}
