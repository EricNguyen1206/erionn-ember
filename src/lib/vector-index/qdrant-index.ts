import { QdrantClient } from '@qdrant/js-client-rest';
import { v4 as uuidv4 } from 'uuid';
import VectorIndex from './interface.js';
import { DistanceMetric, SearchResult } from '../../types/index.js';

/**
 * Qdrant Vector Index Implementation
 * Uses Qdrant Cloud or Local via REST API
 */
export class QdrantVectorIndex extends VectorIndex {
  private client: QdrantClient;
  private collectionName: string;
  private idMap: Map<number, string>; // Maps internal integer ID to Qdrant UUID
  private reverseIdMap: Map<string, number>; // Maps Qdrant UUID back to internal integer ID
  private nextId: number = 0;

  constructor(dim: number, maxElements: number, space: DistanceMetric = 'cosine') {
    super(dim, maxElements, space);

    const url = process.env.QDRANT_URL;
    const apiKey = process.env.QDRANT_API_KEY;

    if (!url) {
      throw new Error('QDRANT_URL environment variable is missing.');
    }

    this.client = new QdrantClient({
      url: url as string,
      apiKey: apiKey as string | undefined,
    });

    this.collectionName = process.env.QDRANT_COLLECTION_NAME ?? 'erion_cache';
    this.idMap = new Map();
    this.reverseIdMap = new Map();
  }

  /**
   * Initializes the collection in Qdrant if it doesn't exist
   */
  async init(): Promise<void> {
    const collections = await this.client.getCollections();
    const exists = collections.collections.some(c => c.name === this.collectionName);

    if (!exists) {
      let distance = 'Cosine';
      if (this.space === 'l2') distance = 'Euclid';
      else if (this.space === 'ip') distance = 'Dot';

      await this.client.createCollection(this.collectionName, {
        vectors: {
          size: this.dim,
          distance: distance as any,
        },
      });
    } else {
      // If collection exists, we might need to sync the internal IDs, 
      // but since this is a Cache starting fresh or loading from a state, 
      // we assume the caller manages the state or it's a fresh start.
      // For a robust implementation we might query existing vectors.
    }
  }

  /**
   * Add vector to index
   * @param vector - Vector to add
   * @param id - Item ID (optional internal int ID)
   * @returns internal int ID of added item
   */
  async addItem(vector: number[], id?: number): Promise<number> {
    let internalId = id;
    if (internalId === undefined) {
      internalId = this.nextId++;
    } else if (internalId >= this.nextId) {
      this.nextId = internalId + 1;
    }

    // Determine UUID for Qdrant
    let qdrantId = this.idMap.get(internalId);
    if (!qdrantId) {
      qdrantId = uuidv4();
      this.idMap.set(internalId, qdrantId);
      this.reverseIdMap.set(qdrantId, internalId);
    }

    await this.client.upsert(this.collectionName, {
      wait: true,
      points: [
        {
          id: qdrantId as string,
          vector: vector,
          payload: { internalId },
        }
      ]
    });

    return internalId;
  }

  /**
   * Search for nearest neighbors
   */
  async search(queryVector: number[], k: number = 10): Promise<SearchResult[]> {
    const results = await this.client.search(this.collectionName, {
      vector: queryVector,
      limit: k,
      with_payload: true,
    });

    return results.map(res => {
      // Qdrant typically returns "score" which is similarity for Cosine, not distance.
      // But we map it based on metric
      let distance = res.score;
      if (this.space === 'cosine') {
        distance = 1 - res.score; // Convert similarity to distance
      }

      const internalId = res.payload?.internalId as number;

      return {
        id: internalId,
        distance,
      };
    });
  }

  /**
   * Save index - Qdrant is persistent, no-op for local file
   */
  async save(path: string): Promise<void> {
    // No-op
  }

  /**
   * Load index - Qdrant is persistent, no-op for local file
   */
  async load(path: string): Promise<void> {
    // No-op
  }

  /**
   * Destroy index
   */
  destroy(): void {
    // No specific close required for JS Rest client typically
  }

  /**
   * Get total number of items
   */
  async getCount(): Promise<number> {
    const info = await this.client.getCollection(this.collectionName);
    return info.points_count || 0;
  }
}

export default QdrantVectorIndex;
