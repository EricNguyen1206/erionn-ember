import VectorIndex from './interface.js';
import { VectorIndexConfig, VectorBackend, DistanceMetric } from '../../types/index.js';

/**
 * Create vector index instance
 * @param options - Configuration options
 * @returns Vector index instance
 */
export async function createVectorIndex(options: VectorIndexConfig): Promise<VectorIndex> {
  const { dim, maxElements, space = 'cosine' } = options;

  const backend: VectorBackend = options.backend ?? (process.env.VECTOR_INDEX_BACKEND as VectorBackend) ?? 'annoy';

  const supportedBackends: VectorBackend[] = ['annoy', 'hnsw', 'turso', 'qdrant'];
  if (!supportedBackends.includes(backend)) {
    throw new Error(`Unknown vector index backend: ${backend}. Supported: ${supportedBackends.join(', ')}`);
  }

  if (backend === 'annoy') {
    const { default: AnnoyVectorIndex } = await import('./annoy-index.js');
    return new AnnoyVectorIndex(dim, maxElements, space as DistanceMetric);
  }

  if (backend === 'hnsw') {
    try {
      const { default: HNSWVectorIndex } = await import('./hnsw-index.js');
      return new HNSWVectorIndex(dim, maxElements, space as DistanceMetric);
    } catch (err) {
      const error = err as Error;
      throw new Error(
        `hnswlib-node not available. ` +
        `Install C++ build tools or use Annoy.js backend (VECTOR_INDEX_BACKEND=annoy). ` +
        `Original error: ${error.message}`
      );
    }
  }

  if (backend === 'turso') {
    const { default: TursoVectorIndex } = await import('./turso-index.js');
    const index = new TursoVectorIndex(dim, maxElements, space as DistanceMetric);
    await index.init();
    return index;
  }

  if (backend === 'qdrant') {
    const { default: QdrantVectorIndex } = await import('./qdrant-index.js');
    const index = new QdrantVectorIndex(dim, maxElements, space as DistanceMetric);
    await index.init();
    return index;
  }

  throw new Error(`Unreachable: unknown backend ${backend}`);
}

export { VectorIndex };
export default createVectorIndex;
