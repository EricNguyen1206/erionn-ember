// Covers: ZADD, ZREMRANGEBYSCORE, ZCARD, ZRANGE
// Context: Online Game — global XP leaderboard, item price history, action rate limiting
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
    await client.del([
      'game:leaderboard:global_xp',
      'game:market:item_history',
      'player:1001:action_ratelimit',
    ])
  })

  // Test 1: ZADD + ZCARD basic operations
  it('ZADD + ZCARD basic — global XP leaderboard', async () => {
    await client.zAdd('game:leaderboard:global_xp', [
      { score: 15000, value: 'ShadowWalker' },
      { score: 20000, value: 'LightBringer' },
      { score: 12000, value: 'FireMage' },
    ])
    const count = await client.zCard('game:leaderboard:global_xp')
    expect(count).toBe(3)
  })

  // Test 2: ZADD ignores duplicate member (updates score)
  it('ZADD ignores duplicate member — update player XP', async () => {
    await client.zAdd('game:leaderboard:global_xp', [{ score: 1000, value: 'ShadowWalker' }])
    const added = await client.zAdd('game:leaderboard:global_xp', [{ score: 3000, value: 'ShadowWalker' }])
    expect(added).toBe(0)
    const count = await client.zCard('game:leaderboard:global_xp')
    expect(count).toBe(1)
  })

  // Test 3: ZREMRANGEBYSCORE removes entries in score range
  it('ZREMRANGEBYSCORE removes entries — clean up old market history', async () => {
    const now = Date.now()
    await client.zAdd('game:market:item_history', [
      { score: now - 5000, value: 'sale-1' },
      { score: now - 4000, value: 'sale-2' },
      { score: now - 2000, value: 'sale-3' },
      { score: now - 1000, value: 'sale-4' },
    ])
    const removed = await client.zRemRangeByScore('game:market:item_history', 0, now - 3500)
    expect(removed).toBe(2)
    const remaining = await client.zCard('game:market:item_history')
    expect(remaining).toBe(2)
  })

  // Test 4: ZRANGE returns members in ascending score order
  it('ZRANGE returns members in score order — leaderboard ranking (lowest to highest)', async () => {
    await client.zAdd('game:leaderboard:global_xp', [
      { score: 5000, value: 'NovicePlayer' },
      { score: 1000, value: 'Newbie' },
      { score: 15000, value: 'ProPlayer' },
    ])
    const range = await client.zRange('game:leaderboard:global_xp', 0, -1)
    expect(range).toEqual(['Newbie', 'NovicePlayer', 'ProPlayer'])
  })

  // Test 5: Sliding window rate limit pattern (sorted sets)
  it('Sliding window rate limit — limit player skill usage', async () => {
    const key = 'player:1001:action_ratelimit'
    const windowMs = 60000
    const maxActions = 5
    const now = Date.now()

    for (let i = 0; i < 5; i++) {
      await client.zAdd(key, [{ score: now - i * 1000, value: `skill-use-${now - i * 1000}` }])
    }

    const count = await client.zCard(key)
    expect(count).toBe(5)
    expect(count).toBeLessThanOrEqual(maxActions)
  })
})