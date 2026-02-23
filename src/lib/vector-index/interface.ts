import { DistanceMetric, SearchResult } from '../../types/index.js';

/**
 * VectorIndex Interface
 * Abstract interface for vector similarity search implementations
 */
export abstract class VectorIndex {
  dim: number;
  maxElements: number;
  space: DistanceMetric;

  constructor(dim: number, maxElements: number, space: DistanceMetric = 'cosine') {
    if (new.target === VectorIndex) {
      throw new Error('VectorIndex is abstract - cannot instantiate directly');
    }
    this.dim = dim;
    this.maxElements = maxElements;
    this.space = space;
  }

  /**
   * Add vector to index
   * @param vector - Vector to add
   * @param id - Item ID (optional)
   * @returns ID of added item
   */
  abstract addItem(vector: number[], id?: number): number | Promise<number>;

  /**
   * Search for nearest neighbors
   * @param queryVector - Query vector
   * @param k - Number of results
   * @returns Search results
   */
  abstract search(queryVector: number[], k?: number): SearchResult[] | Promise<SearchResult[]>;

  /**
   * Save index to file
   * @param path - File path
   */
  abstract save(path: string): Promise<void>;

  /**
   * Load index from file
   * @param path - File path
   */
  abstract load(path: string): Promise<void>;

  /**
   * Destroy index and free memory
   */
  abstract destroy(): void;

  /**
   * Get number of items in index
   * @returns Number of items
   */
  abstract getCount(): number | Promise<number>;
}

export default VectorIndex;
