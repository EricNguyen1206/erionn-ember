// Presence pattern — Full simulation of erion-raven's online status tracking
// Combines: SADD/SREM (online set) + HSET (status details) + EXPIRE (heartbeat TTL)
const { createRedisClient } = require('./redis')

describe('Presence Pattern (erion-raven)', () => {
  let client

  beforeAll(async () => {
    client = await createRedisClient()
  })

  afterAll(async () => {
    await client.quit()
  })

  afterEach(async () => {
    await client.del([
      'test:presence:online_users',
      'test:presence:user:1:status',
      'test:presence:user:2:status',
      'test:presence:user:3:status'
    ])
  })

  it('user comes online', async () => {
    const userId = 'user-1'
    const now = Date.now()

    await client.sAdd('test:presence:online_users', userId)
    await client.hSet(`test:presence:${userId}:status`, {
      userId,
      status: 'online',
      lastSeen: now.toString(),
    })
    await client.expire(`test:presence:${userId}:status`, 60)

    const isOnline = await client.sIsMember('test:presence:online_users', userId)
    expect(isOnline).toBe(true)

    const status = await client.hGetAll(`test:presence:${userId}:status`)
    expect(status.status).toBe('online')
    expect(status.userId).toBe(userId)

    const ttl = await client.ttl(`test:presence:${userId}:status`)
    expect(ttl).toBeGreaterThan(0)
  })

  it('user goes offline', async () => {
    const userId = 'user-1'

    // Set online first
    await client.sAdd('test:presence:online_users', userId)
    await client.hSet(`test:presence:${userId}:status`, 'status', 'online')

    // Transition to offline (atomic)
    await client
      .multi()
      .sRem('test:presence:online_users', userId)
      .hSet(`test:presence:${userId}:status`, 'status', 'offline')
      .expire(`test:presence:${userId}:status`, 86400)
      .exec()

    const isOnline = await client.sIsMember('test:presence:online_users', userId)
    expect(isOnline).toBe(false)

    const status = await client.hGet(`test:presence:${userId}:status`, 'status')
    expect(status).toBe('offline')
  })

  it('get all online users', async () => {
    await client.sAdd('test:presence:online_users', 'user-1')
    await client.sAdd('test:presence:online_users', 'user-2')
    await client.sAdd('test:presence:online_users', 'user-3')

    const onlineUsers = await client.sMembers('test:presence:online_users')
    expect(onlineUsers.sort()).toEqual(['user-1', 'user-2', 'user-3'])
  })

  it('heartbeat renews TTL', async () => {
    const userId = 'user-1'

    await client.hSet(`test:presence:${userId}:status`, 'status', 'online')
    await client.expire(`test:presence:${userId}:status`, 60)

    // Simulate heartbeat — renew TTL
    await client.expire(`test:presence:${userId}:status`, 60)

    const ttl = await client.ttl(`test:presence:${userId}:status`)
    expect(ttl).toBeGreaterThan(55)
  })

  it('check if specific friend is online before broadcasting', async () => {
    await client.sAdd('test:presence:online_users', 'user-1')

    const friendOnline = await client.sIsMember('test:presence:online_users', 'user-1')
    const friendOffline = await client.sIsMember('test:presence:online_users', 'user-99')

    expect(friendOnline).toBe(true)
    expect(friendOffline).toBe(false)
  })
})
