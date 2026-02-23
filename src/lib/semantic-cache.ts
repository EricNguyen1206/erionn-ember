import {
  CacheConfig,
  CacheOptions,
  CacheResult,
  QueryOptions,
  CacheMetadata,
  CacheStats,
  InternalStatistics,
  SearchResult,
} from '../types/index.js';
import { createVectorIndex, VectorIndex } from './vector-index/factory.js';
import Quantizer from './quantizer.js';
import Compressor from './compressor.js';
import Normalizer from './normalizer.js';
import MetadataStore from './metadata-store.js';

/**
 * Semantic Cache - High-performance cache for LLM queries with vector search
 */
export interface SemanticCacheDependencies {
  quantizer?: Quantizer;
  compressor?: Compressor;
  normalizer?: Normalizer;
  metadataStore?: MetadataStore;
}

export class SemanticCache {
  readonly dim: number;
  readonly maxElements: number;
  readonly similarityThreshold: number;
  readonly memoryLimit: string;
  readonly defaultTTL: number;

  private quantizer: Quantizer;
  private compressor: Compressor;
  private normalizer: Normalizer;
  private metadataStore: MetadataStore;
  private _statistics: InternalStatistics;
  private index: VectorIndex | null = null;
  private indexPromise: Promise<VectorIndex> | null = null;

  constructor(options: CacheConfig = {}, deps: SemanticCacheDependencies = {}) {
    this.dim = options.dim ?? 1536;
    this.maxElements = options.maxElements ?? 100000;
    this.similarityThreshold = options.similarityThreshold ?? 0.85;
    this.memoryLimit = options.memoryLimit ?? '1gb';
    this.defaultTTL = options.defaultTTL ?? 3600;

    this.quantizer = deps.quantizer ?? new Quantizer();
    this.compressor = deps.compressor ?? new Compressor();
    this.normalizer = deps.normalizer ?? new Normalizer();
    this.metadataStore = deps.metadataStore ?? new MetadataStore({ maxSize: this.maxElements });

    this._statistics = {
      hits: 0,
      misses: 0,
      totalQueries: 0,
      savedTokens: 0,
      savedUsd: 0,
    };

    this.indexPromise = this._initIndex();
  }

  /**
   * Initialize vector index asynchronously
   * @private
   */
  private async _initIndex(): Promise<VectorIndex> {
    this.index = await createVectorIndex({
      dim: this.dim,
      maxElements: this.maxElements,
      space: 'cosine',
    });
    return this.index;
  }

  /**
   * Ensure index is initialized
   * @private
   */
  private async _ensureIndex(): Promise<void> {
    if (!this.index && this.indexPromise) {
      await this.indexPromise;
    }
  }

  /**
   * Track savings from a cache hit
   * @param tokens - Number of tokens saved
   * @param usd - USD amount saved
   */
  trackSavings(tokens: number, usd: number): void {
    this._statistics.savedTokens += tokens;
    this._statistics.savedUsd += usd;
  }

  /**
   * Query cache
   * @param prompt - Query prompt
   * @param embedding - Query embedding vector
   * @param options - Query options
   * @returns Cache result or null
   */
  async get(
    prompt: string,
    embedding: number[] | null = null,
    options: QueryOptions = {}
  ): Promise<CacheResult | null> {
    await this._ensureIndex();

    this._statistics.totalQueries++;
    const minSimilarity = options.minSimilarity ?? this.similarityThreshold;

    const normalized = this.normalizer.normalize(prompt);
    const promptHash = this.normalizer.hash(normalized, true);

    // Check exact match first
    const exactMatch = this.metadataStore.findByPromptHash(promptHash);
    if (exactMatch) {
      this._statistics.hits++;
      const response = this._decompressResponse(exactMatch);
      return {
        response,
        similarity: 1.0,
        isExactMatch: true,
        cachedAt: new Date(exactMatch.createdAt),
        metadata: exactMatch,
      };
    }

    // If no embedding provided, can't do semantic search
    if (!embedding) {
      this._statistics.misses++;
      return null;
    }

    // Search similar vectors
    const quantizedQuery = this.quantizer.quantize(embedding);
    const baseK = 5;
    const maxK = Math.min(this.maxElements, 50);
    let k = Math.min(baseK, this.maxElements);
    const inspected = new Set<number>();

    // Find best match above threshold
    while (k > 0 && this.index) {
      const searchResults = await this.index.search(quantizedQuery, k);
      let staleCandidate = false;

      for (const result of searchResults) {
        if (inspected.has(result.id)) {
          continue;
        }
        inspected.add(result.id);

        const similarity = 1 - result.distance;

        if (similarity >= minSimilarity) {
          const metadata = this.metadataStore.get(result.id.toString());
          if (metadata) {
            this._statistics.hits++;
            const response = this._decompressResponse(metadata);
            return {
              response,
              similarity,
              isExactMatch: false,
              cachedAt: new Date(metadata.createdAt),
              metadata,
            };
          }
          staleCandidate = true;
        }
      }

      if (!staleCandidate || k >= maxK || searchResults.length < k) {
        break;
      }

      k = Math.min(maxK, k + baseK);
    }

    this._statistics.misses++;
    return null;
  }

  /**
   * Add entry to cache
   * @param prompt - Original prompt
   * @param response - LLM response
   * @param embedding - Vector embedding
   * @param options - Cache options (e.g., ttl)
   */
  async set(
    prompt: string,
    response: string,
    embedding: number[],
    options: CacheOptions = {}
  ): Promise<void> {
    await this._ensureIndex();

    if (!this.index) {
      throw new Error('Vector index not initialized');
    }

    const normalized = this.normalizer.normalize(prompt);
    const promptHash = this.normalizer.hash(normalized, true);

    const compressedPrompt = this.compressor.compress(prompt);
    const compressedResponse = this.compressor.compress(response);

    const quantizedVector = this.quantizer.quantize(embedding);

    const vectorId = await this.index.addItem(quantizedVector);

    const id = vectorId.toString();
    const metadata: CacheMetadata = {
      id,
      vectorId,
      promptHash,
      normalizedPrompt: normalized,
      compressedPrompt,
      compressedResponse,
      originalPromptSize: Buffer.byteLength(prompt, 'utf8'),
      originalResponseSize: Buffer.byteLength(response, 'utf8'),
      compressedPromptSize: compressedPrompt.length,
      compressedResponseSize: compressedResponse.length,
      createdAt: Date.now(),
      lastAccessed: Date.now(),
      accessCount: 0,
    };

    const ttl = options.ttl ?? this.defaultTTL;
    this.metadataStore.set(id, metadata, ttl);
  }

  /**
   * Delete entry from cache
   * @param prompt - Prompt to delete
   * @returns Whether deletion was successful
   */
  delete(prompt: string): boolean {
    const normalized = this.normalizer.normalize(prompt);
    const promptHash = this.normalizer.hash(normalized, true);
    const metadata = this.metadataStore.findByPromptHash(promptHash);

    if (metadata) {
      return this.metadataStore.delete(metadata.id);
    }
    return false;
  }

  /**
   * Get cache statistics
   * @returns Cache statistics object
   */
  getStats(): CacheStats {
    const storeStats = this.metadataStore.stats();
    const hitRate =
      this._statistics.totalQueries > 0
        ? this._statistics.hits / this._statistics.totalQueries
        : 0;

    return {
      totalEntries: storeStats.totalEntries,
      memoryUsage: {
        vectors: storeStats.totalEntries * this.dim,
        metadata: storeStats.totalCompressedSize,
        total: storeStats.totalEntries * this.dim + storeStats.totalCompressedSize,
      },
      compressionRatio: this._calculateCompressionRatio(),
      cacheHits: this._statistics.hits,
      cacheMisses: this._statistics.misses,
      hitRate: hitRate.toFixed(4),
      totalQueries: this._statistics.totalQueries,
      savedTokens: this._statistics.savedTokens,
      savedUsd: Number(this._statistics.savedUsd.toFixed(5)),
    };
  }

  /**
   * Clear all cache entries
   */
  async clear(): Promise<void> {
    this.metadataStore.clear();
    this.index = await createVectorIndex({
      dim: this.dim,
      maxElements: this.maxElements,
      space: 'cosine',
    });
    this._statistics = { hits: 0, misses: 0, totalQueries: 0, savedTokens: 0, savedUsd: 0 };
  }

  /**
   * Save cache to disk
   * @param path - Directory path
   */
  async save(path: string): Promise<void> {
    await this._ensureIndex();

    if (!this.index) {
      throw new Error('Vector index not initialized');
    }

    const fs = await import('fs/promises');

    await this.index.save(`${path}/index.bin`);

    const store = Array.from(this.metadataStore.entries()).map(([id, data]) => ({
      ...data,
      compressedPrompt: data.compressedPrompt.toString('base64'),
      compressedResponse: data.compressedResponse.toString('base64'),
    }));

    const metadata = {
      stats: this._statistics,
      store,
      config: {
        dim: this.dim,
        maxElements: this.maxElements,
        similarityThreshold: this.similarityThreshold,
      },
    };
    await fs.writeFile(`${path}/metadata.json`, JSON.stringify(metadata, null, 2));
  }

  /**
   * Load cache from disk
   * @param path - Directory path
   */
  async load(path: string): Promise<void> {
    await this._ensureIndex();

    if (!this.index) {
      throw new Error('Vector index not initialized');
    }

    const fs = await import('fs/promises');

    await this.index.load(`${path}/index.bin`);

    const data = await fs.readFile(`${path}/metadata.json`, 'utf8');
    const metadata: {
      stats: InternalStatistics;
      store: Array<CacheMetadata & { compressedPrompt: string; compressedResponse: string }>;
      config: CacheConfig;
    } = JSON.parse(data);

    this.metadataStore.clear();
    const now = Date.now();
    for (const entry of metadata.store) {
      if (entry.expiresAt && entry.expiresAt <= now) {
        continue;
      }

      let ttl: number | undefined;
      if (entry.expiresAt) {
        const remainingMs = entry.expiresAt - now;
        if (remainingMs <= 0) {
          continue;
        }
        ttl = Math.ceil(remainingMs / 1000);
      }

      const restoredEntry: CacheMetadata = {
        ...entry,
        compressedPrompt: Buffer.from(entry.compressedPrompt, 'base64'),
        compressedResponse: Buffer.from(entry.compressedResponse, 'base64'),
      };

      this.metadataStore.set(entry.id, restoredEntry, ttl);
    }

    // Merge with defaults to handle older metadata missing fields
    const defaultStats: InternalStatistics = {
      hits: 0,
      misses: 0,
      totalQueries: 0,
      savedTokens: 0,
      savedUsd: 0,
    };
    this._statistics = { ...defaultStats, ...metadata.stats };
  }

  /**
   * Destroy cache and free resources
   */
  destroy(): void {
    if (this.index) {
      this.index.destroy();
    }
    this.metadataStore.clear();
  }

  /**
   * Decompress response from metadata
   * @private
   */
  private _decompressResponse(metadata: CacheMetadata): string {
    return this.compressor.decompress(
      metadata.compressedResponse,
      metadata.originalResponseSize
    );
  }

  /**
   * Calculate overall compression ratio
   * @private
   */
  private _calculateCompressionRatio(): string {
    let totalOriginal = 0;
    let totalCompressed = 0;

    for (const data of this.metadataStore.values()) {
      totalOriginal += data.originalResponseSize;
      totalCompressed += data.compressedResponseSize;
    }

    return totalOriginal > 0 ? (totalCompressed / totalOriginal).toFixed(2) : '0';
  }
}

export default SemanticCache;
