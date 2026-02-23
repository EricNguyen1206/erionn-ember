import { config } from 'dotenv';
config();

import { createVectorIndex } from './src/lib/vector-index/factory.js';

async function testQdrant() {
  console.log('Testing Qdrant connection...');
  console.log(`URL: ${process.env.QDRANT_URL}`);
  console.log(`API Key set: ${!!process.env.QDRANT_API_KEY}`);

  if (!process.env.QDRANT_URL) {
    console.error('Missing QDRANT_URL in .env');
    process.exit(1);
  }

  process.env.VECTOR_INDEX_BACKEND = 'qdrant';

  try {
    const index = await createVectorIndex({
      dim: 3, // Small dimension for testing
      maxElements: 100,
      space: 'cosine'
    });

    console.log('✅ Index created/initialized successfully');

    const v1 = [0.1, 0.2, 0.3];
    const v2 = [0.9, 0.8, 0.7];
    const v3 = [0.1, 0.21, 0.29]; // Similar to v1

    console.log('Adding vectors...');
    const id1 = await index.addItem(v1);
    const id2 = await index.addItem(v2);
    const id3 = await index.addItem(v3);
    console.log(`Added vectors with internal IDs: ${id1}, ${id2}, ${id3}`);

    const count = await index.getCount();
    console.log(`Total vectors in collection: ${count}`);

    console.log('\nSearching for vector similar to v1 [0.1, 0.2, 0.3]...');
    const results = await index.search([0.1, 0.2, 0.3], 2);

    console.log('Search Results:');
    results.forEach(res => {
      console.log(` - ID: ${res.id}, Distance: ${res.distance.toFixed(4)}`);
    });

    console.log('\n🎉 Qdrant integration test passed!');
    process.exit(0);

  } catch (err) {
    console.error('❌ Test failed:', err);
    process.exit(1);
  }
}

testQdrant();
