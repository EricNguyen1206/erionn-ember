// Rate Limiting pattern — Full simulation of erion-raven's sliding window rate limiter
// Uses: ZADD, ZREMRANGEBYSCORE, ZCARD, EXPIRE
const { createRedisClient } = require('./redis')

describe('Rate Limiting Pattern (erion-raven)', () => {
  let client

  beforeAll(async () => {
    client = await createRedisClient()
  })

  afterAll(async () => {
    await client.quit()
  })

  afterEach(async () => {
    await client.del([
      'test:ratelimit:user:user-1:api',
      'test:ratelimit:ip:127.0.0.1:api',
      'test:ratelimit:ws:user-1:messages'
    ])
  })

  it('allows requests within limit', async () => {
    const key = 'test:ratelimit:user:user-1:api'
    const windowMs = 60000
    const maxRequests = 5
    const now = Date.now()
    const windowStart = now - windowMs

    // Add request
    await client.zAdd(key, [{ score: now, value: now.toString() }])
    await client.zRemRangeByScore(key, 0, windowStart.toString())
    await client.expire(key, Math.ceil(windowMs / 1000))

    const count = await client.zCard(key)
    expect(count).toBeLessThanOrEqual(maxRequests)
    expect(count).toBe(1)
  })

  it('blocks requests exceeding limit', async () => {
    const key = 'test:ratelimit:user:user-1:api'
    const windowMs = 60000
    const maxRequests = 3
    const now = Date.now()

    // Fill up to limit
    for (let i = 0; i < maxRequests; i++) {
      const ts = now - i * 1000
      await client.zAdd(key, [{ score: ts, value: ts.toString() }])
    }

    const count = await client.zCard(key)
    expect(count).toBe(maxRequests)
    // Request should be rejected
    expect(count >= maxRequests).toBe(true)
  })

  it('old entries are cleaned up by ZREMRANGEBYSCORE', async () => {
    const key = 'test:ratelimit:user:user-1:api'
    const windowMs = 60000
    const now = Date.now()
    const windowStart = now - windowMs

    // Add old entries
    await client.zAdd(key, [
      { score: now - 120000, value: 'old-1' }, // 2 min ago
      { score: now - 90000, value: 'old-2' },  // 1.5 min ago
      { score: now, value: now.toString() },    // now
    ])

    // Cleanup old entries
    const removed = await client.zRemRangeByScore(key, 0, windowStart.toString())
    expect(removed).toBe(2)

    const remaining = await client.zCard(key)
    expect(remaining).toBe(1)
  })

  it('IP-based rate limiting (same pattern, different key)', async () => {
    const key = 'test:ratelimit:ip:127.0.0.1:api'
    const windowMs = 60000
    const now = Date.now()

    await client.zAdd(key, [{ score: now, value: now.toString() }])
    const count = await client.zCard(key)
    expect(count).toBe(1)
  })

  it('WebSocket message rate limiting', async () => {
    const key = 'test:ratelimit:ws:user-1:messages'
    const windowMs = 60000
    const maxMessages = 30
    const now = Date.now()

    // Simulate burst of messages
    for (let i = 0; i < 10; i++) {
      await client.zAdd(key, [{ score: now + i, value: `${now + i}` }])
    }

    const count = await client.zCard(key)
    expect(count).toBe(10)
    expect(count).toBeLessThanOrEqual(maxMessages)
  })

  it('full sliding window check flow', async () => {
    const key = 'test:ratelimit:user:user-1:api'
    const windowMs = 60000
    const maxRequests = 5
    const now = Date.now()
    const windowStart = now - windowMs

    // Step 1: Clean up old entries
    await client.zRemRangeByScore(key, 0, windowStart.toString())

    // Step 2: Check current count
    let count = await client.zCard(key)
    expect(count).toBe(0)

    // Step 3: Add current request
    await client.zAdd(key, [{ score: now, value: now.toString() }])

    // Step 4: Set expiry
    await client.expire(key, Math.ceil(windowMs / 1000))

    // Step 5: Verify
    count = await client.zCard(key)
    expect(count).toBe(1)
    expect(count).toBeLessThanOrEqual(maxRequests)
  })
})
