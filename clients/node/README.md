# Ember Node.js Client

The official Node.js SDK for [Ember](https://github.com/EricNguyen1206/erionn-ember) - High-performance semantic cache service for LLM applications.

## Installation

You can install the package using npm or yarn:

```bash
npm install ember-client
# or
yarn add ember-client
```

This release publishes the package as `ember-client`.

Version `3.0.0` is the intentional major release that aligns the Node client metadata with the server v3 release.

## Quick Start
TypeScript / ES Modules:

```typescript
import { Ember } from 'ember-client';

async function main() {
  // Initialize the client
  const cache = new Ember('localhost', 8080); // defaults to localhost:8080

  try {
    // 1. Set a cache entry (e.g., an LLM response)
    const setRes = await cache.set({
      prompt: 'What is Go?',
      response: 'Go is a compiled, statically typed language developed by Google.',
      ttl: 3600 // Time to live in seconds
    });
    console.log('Set Result:', setRes);

    // 2. Retrieve with exact match
    const exactRes = await cache.get({
      prompt: 'What is Go?',
      similarity_threshold: 1.0
    });
    console.log('Exact Match:', exactRes);

    // 3. Retrieve with semantic match
    const semanticRes = await cache.get({
      prompt: 'Can you tell me about the Go language?',
      similarity_threshold: 0.8
    });
    console.log('Semantic Match:', semanticRes);

    // 4. View cache statistics
    const stats = await cache.stats();
    console.log('Stats:', stats);

  } catch (error) {
    console.error('Ember Error:', error);
  }
}

main();
```

CommonJS:
```javascript
const { Ember } = require('ember-client');
const cache = new Ember();
```

## Running tests

```bash
npm install
npm test
```
