import crypto from 'crypto';

/**
 * Prompt Normalizer - Normalizes text for deduplication
 */
export class Normalizer {
  constructor() {
  }

  /**
   * Normalize text for caching
   * - Lowercase
   * - Trim
   * - Remove extra spaces
   * @param text - Input text
   * @returns Normalized text
   */
  normalize(text: string): string {
    if (!text || typeof text !== 'string') {
      return '';
    }

    return text.toLowerCase().trim().replace(/\s+/g, ' ');
  }

  /**
   * Generate hash for deduplication
   * Uses crypto sha256 for compatibility with bun standalone compilation
   * @param text - Input text (will be normalized if not already)
   * @param alreadyNormalized - If true, assumes text is already normalized
   * @returns Hash string
   */
  hash(text: string, alreadyNormalized: boolean = false): string {
    const normalized = alreadyNormalized ? text : this.normalize(text);
    const hasher = crypto.createHash('sha256');
    hasher.update(Buffer.from(normalized, 'utf8'));
    return hasher.digest('hex');
  }
}

export default Normalizer;
