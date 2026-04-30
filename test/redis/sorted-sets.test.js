// Sorted Sets — erion-raven uses: ZADD, ZREMRANGEBYSCORE, ZCARD
// Used for sliding window rate limiting
const { createRedisClient } = require('./redis')

describe('Sorted Sets — ZADD / ZREMRANGEBYSCORE / ZCARD / ZRANGE', () => {
  let client

  beforeAll(async () => {
    client = await createRedisClient()
  })

  afterAll(async () => {
    await client.quit()
  })

  afterEach(async () => {
    await client.del(['test:zset:1', 'test:zset:ratelimit'])
  })

  it('ZADD + ZCARD basic', async () => {
    await client.zAdd('test:zset:1', [
      { score: 1, value: 'a' },
      { score: 2, value: 'b' },
      { score: 3, value: 'c' },
    ])
    const count = await client.zCard('test:zset:1')
    expect(count).toBe(3)
  })

  it('ZADD ignores duplicate member (updates score)', async () => {
    await client.zAdd('test:zset:1', [{ score: 1, value: 'a' }])
    const added = await client.zAdd('test:zset:1', [{ score: 5, value: 'a' }])
    expect(added).toBe(0)
    const count = await client.zCard('test:zset:1')
    expect(count).toBe(1)
  })

  it('ZREMRANGEBYSCORE removes entries in score range (erion-raven rate limit cleanup)', async () => {
    await client.zAdd('test:zset:1', [
      { score: 100, value: 'old-1' },
      { score: 200, value: 'old-2' },
      { score: 500, value: 'new-1' },
      { score: 600, value: 'new-2' },
    ])
    // Remove all entries with score < 400
    const removed = await client.zRemRangeByScore('test:zset:1', 0, 399)
    expect(removed).toBe(2)
    const remaining = await client.zCard('test:zset:1')
    expect(remaining).toBe(2)
  })

  it('ZRANGE returns members in score order', async () => {
    await client.zAdd('test:zset:1', [
      { score: 3, value: 'c' },
      { score: 1, value: 'a' },
      { score: 2, value: 'b' },
    ])
    const range = await client.zRange('test:zset:1', 0, -1)
    expect(range).toEqual(['a', 'b', 'c'])
  })

  it('Sliding window rate limit simulation (erion-raven pattern)', async () => {
    const key = 'test:zset:ratelimit'
    const windowMs = 60000 // 1 minute window
    const maxRequests = 5
    const now = Date.now()
    const windowStart = now - windowMs

    // Add 5 requests
    for (let i = 0; i < 5; i++) {
      await client.zAdd(key, [{ score: now - i * 1000, value: `${now - i * 1000}` }])
    }

    // Check count (simulate rate limit check)
    const count = await client.zCard(key)
    expect(count).toBe(5)
    expect(count).toBeLessThanOrEqual(maxRequests)
  })
})
