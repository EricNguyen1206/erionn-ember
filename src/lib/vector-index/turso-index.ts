import { createClient, Client } from '@libsql/client';
import VectorIndex from './interface.js';
import { DistanceMetric, SearchResult } from '../../types/index.js';

/**
 * Turso (libSQL) Vector Index Implementation
 * Uses native vector search in libSQL
 */
export class TursoVectorIndex extends VectorIndex {
  private client: Client;
  private tableName: string;

  constructor(dim: number, maxElements: number, space: DistanceMetric = 'cosine') {
    super(dim, maxElements, space);

    const url = process.env.TURSO_URL;
    const authToken = process.env.TURSO_AUTH_TOKEN;

    if (!url) {
      throw new Error('TURSO_URL environment variable is missing.');
    }

    this.client = createClient({
      url,
      authToken,
    });

    this.tableName = process.env.TURSO_TABLE_NAME ?? 'vector_index';
  }

  async init(): Promise<void> {
    // Create vector table if not exists using F32_BLOB format
    // libSQL vector search syntax
    await this.client.execute(`
      CREATE TABLE IF NOT EXISTS ${this.tableName} (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        embedding F32_BLOB(${this.dim})
      )
    `);
  }

  /**
   * Add vector to index
   */
  async addItem(vector: number[], id?: number): Promise<number> {
    // Convert float array to Float32Array then to Buffer
    const f32Array = new Float32Array(vector);
    const buffer = Buffer.from(f32Array.buffer);

    let result;
    if (id !== undefined) {
      result = await this.client.execute({
        sql: `INSERT INTO ${this.tableName} (id, embedding) VALUES (?, ?) ON CONFLICT(id) DO UPDATE SET embedding=excluded.embedding`,
        args: [id, buffer.buffer]
      });
      return id;
    } else {
      result = await this.client.execute({
        sql: `INSERT INTO ${this.tableName} (embedding) VALUES (?)`,
        args: [buffer.buffer]
      });
      return Number(result.lastInsertRowid);
    }
  }

  /**
   * Search for nearest neighbors
   */
  async search(queryVector: number[], k: number = 10): Promise<SearchResult[]> {
    const f32Array = new Float32Array(queryVector);
    const buffer = Buffer.from(f32Array.buffer);

    // Using libSQL vector_distance for cosine distance. 
    // vector_distance returns distance, so we order by distance
    const rs = await this.client.execute({
      sql: `SELECT id, vector_distance_cos(embedding, vector(?)) as dist FROM ${this.tableName} ORDER BY dist ASC LIMIT ?`,
      args: [`[${queryVector.join(',')}]`, k]
    });

    return rs.rows.map(row => ({
      id: Number(row.id),
      distance: Number(row.dist)
    }));
  }

  /**
   * Save index - Not needed for Turso as it's cloud persistent
   */
  async save(path: string): Promise<void> {
    // No-op
  }

  /**
   * Load index - Not needed for Turso
   */
  async load(path: string): Promise<void> {
    // No-op
  }

  /**
   * Destroy index and close connections
   */
  destroy(): void {
    if (this.client) {
      this.client.close();
    }
  }

  /**
   * Get total number of items
   */
  async getCount(): Promise<number> {
    const rs = await this.client.execute(`SELECT COUNT(*) as cnt FROM ${this.tableName}`);
    return Number(rs.rows[0].cnt);
  }
}

export default TursoVectorIndex;
