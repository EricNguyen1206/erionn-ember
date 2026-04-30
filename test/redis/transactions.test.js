// Transactions — erion-raven uses: MULTI/EXEC for atomic user status updates
const { createRedisClient } = require('./redis')

describe('Transactions — MULTI / EXEC', () => {
  let client

  beforeAll(async () => {
    client = await createRedisClient()
  })

  afterAll(async () => {
    await client.quit()
  })

  afterEach(async () => {
    await client.del([
      'test:tx:online_users',
      'test:tx:user:1:status',
      'test:tx:user:2:status'
    ])
  })

  it('MULTI/EXEC atomic set operations', async () => {
    const results = await client
      .multi()
      .set('test:tx:user:1:status', 'online')
      .sAdd('test:tx:online_users', 'user-1')
      .exec()

    expect(results).toHaveLength(2)
    expect(results[0]).toEqual(expect.stringMatching(/OK/i))
    expect(results[1]).toBe(1)
  })

  it('MULTI/EXEC atomic hash + set (erion-raven status update pattern)', async () => {
    const results = await client
      .multi()
      .hSet('test:tx:user:1:status', {
        userId: 'user-1',
        status: 'online',
        lastSeen: Date.now().toString(),
      })
      .sAdd('test:tx:online_users', 'user-1')
      .expire('test:tx:user:1:status', 60)
      .exec()

    expect(results).toHaveLength(3)

    // Verify side effects
    const status = await client.hGetAll('test:tx:user:1:status')
    expect(status.status).toBe('online')

    const online = await client.sMembers('test:tx:online_users')
    expect(online).toEqual(['user-1'])

    const ttl = await client.ttl('test:tx:user:1:status')
    expect(ttl).toBeGreaterThan(0)
  })

  it('MULTI/EXEC offline transition (erion-raven offline pattern)', async () => {
    // First, set user online
    await client.sAdd('test:tx:online_users', 'user-1')
    await client.hSet('test:tx:user:1:status', 'status', 'online')

    // Atomic transition to offline
    await client
      .multi()
      .sRem('test:tx:online_users', 'user-1')
      .hSet('test:tx:user:1:status', 'status', 'offline')
      .expire('test:tx:user:1:status', 86400) // 24h TTL for offline
      .exec()

    const online = await client.sMembers('test:tx:online_users')
    expect(online).toEqual([])

    const status = await client.hGet('test:tx:user:1:status', 'status')
    expect(status).toBe('offline')
  })

  it('MULTI/EXEC multiple users atomically', async () => {
    await client
      .multi()
      .sAdd('test:tx:online_users', 'user-1')
      .sAdd('test:tx:online_users', 'user-2')
      .hSet('test:tx:user:1:status', 'status', 'online')
      .hSet('test:tx:user:2:status', 'status', 'online')
      .exec()

    const online = await client.sMembers('test:tx:online_users')
    expect(online.sort()).toEqual(['user-1', 'user-2'])
  })
})
