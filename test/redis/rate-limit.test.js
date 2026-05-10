// Covers: ZADD, ZREMRANGEBYSCORE, ZCARD, EXPIRE (sliding window pattern)
// Context: Online Game — skill cooldowns, chat message throttling, dungeon entry limits
const { createRedisClient } = require('./redis')

describe('Rate Limiting Pattern — Game action throttling', () => {
  let client

  beforeAll(async () => {
    client = await createRedisClient()
  })

  afterAll(async () => {
    await client.quit()
  })

  afterEach(async () => {
    await client.del([
      'game:ratelimit:player:1001:skills',
      'game:ratelimit:ip:192.168.1.1:chat',
      'game:ratelimit:player:1001:dungeons',
    ])
  })

  // Test 1: Allows actions within limit
  it('allows actions within limit — 5 skills per minute', async () => {
    const key = 'game:ratelimit:player:1001:skills'
    const windowMs = 60000
    const maxSkills = 5
    const now = Date.now()
    const windowStart = now - windowMs

    await client.zAdd(key, [{ score: now, value: now.toString() }])
    await client.zRemRangeByScore(key, 0, windowStart.toString())
    await client.expire(key, Math.ceil(windowMs / 1000))

    const count = await client.zCard(key)
    expect(count).toBeLessThanOrEqual(maxSkills)
    expect(count).toBe(1)
  })

  // Test 2: Blocks requests exceeding limit
  it('blocks requests exceeding limit — skill spam prevention', async () => {
    const key = 'game:ratelimit:player:1001:skills'
    const windowMs = 60000
    const maxSkills = 3
    const now = Date.now()

    for (let i = 0; i < maxSkills; i++) {
      const ts = now - i * 1000
      await client.zAdd(key, [{ score: ts, value: ts.toString() }])
    }

    const count = await client.zCard(key)
    expect(count).toBe(maxSkills)
    expect(count >= maxSkills).toBe(true)
  })

  // Test 3: ZREMRANGEBYSCORE cleans up old entries
  it('ZREMRANGEBYSCORE cleans up old entries outside time window', async () => {
    const key = 'game:ratelimit:player:1001:skills'
    const windowMs = 60000
    const now = Date.now()
    const windowStart = now - windowMs

    await client.zAdd(key, [
      { score: now - 120000, value: 'old-skill-1' },
      { score: now - 90000, value: 'old-skill-2' },
      { score: now, value: now.toString() },
    ])

    const removed = await client.zRemRangeByScore(key, 0, windowStart.toString())
    expect(removed).toBe(2)

    const remaining = await client.zCard(key)
    expect(remaining).toBe(1)
  })

  // Test 4: IP-based rate limiting
  it('IP-based rate limiting — prevent chat spam', async () => {
    const key = 'game:ratelimit:ip:192.168.1.1:chat'
    const windowMs = 60000
    const now = Date.now()

    await client.zAdd(key, [{ score: now, value: now.toString() }])
    const count = await client.zCard(key)
    expect(count).toBe(1)
  })

  // Test 5: Dungeon entry rate limit
  it('dungeon entry rate limit — max 10 entries per day', async () => {
    const key = 'game:ratelimit:player:1001:dungeons'
    const windowMs = 86400000 // 24h
    const maxEntries = 10
    const now = Date.now()

    for (let i = 0; i < 8; i++) {
      await client.zAdd(key, [{ score: now + i, value: `entry-${now + i}` }])
    }

    const count = await client.zCard(key)
    expect(count).toBe(8)
    expect(count).toBeLessThanOrEqual(maxEntries)
  })

  // Test 6: Full sliding window check flow
  it('full sliding window check flow', async () => {
    const key = 'game:ratelimit:player:1001:skills'
    const windowMs = 60000
    const maxSkills = 5
    const now = Date.now()
    const windowStart = now - windowMs

    await client.zRemRangeByScore(key, 0, windowStart.toString())

    let count = await client.zCard(key)
    expect(count).toBe(0)

    await client.zAdd(key, [{ score: now, value: now.toString() }])
    await client.expire(key, Math.ceil(windowMs / 1000))

    count = await client.zCard(key)
    expect(count).toBe(1)
    expect(count).toBeLessThanOrEqual(maxSkills)
  })
})