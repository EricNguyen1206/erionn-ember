// Covers: SADD, SREM, SISMEMBER, SMEMBERS, HSET, HGET, HGETALL, EXPIRE, MULTI/EXEC
// Context: Online Game — player presence tracking (online/offline, heartbeat, status)
const { createRedisClient } = require('./redis')

describe('Presence Pattern — Player online/offline tracking', () => {
  let client

  beforeAll(async () => {
    client = await createRedisClient()
  })

  afterAll(async () => {
    await client.quit()
  })

  afterEach(async () => {
    await client.del([
      'game:presence:online_players',
      'game:presence:player:1001:status',
      'game:presence:player:1002:status',
      'game:presence:player:1003:status',
    ])
  })

  // Test 1: Player logs in (goes online)
  it('player logs in — goes online', async () => {
    const playerId = 'player-1001'
    const now = Date.now()

    await client.sAdd('game:presence:online_players', playerId)
    await client.hSet(`game:presence:${playerId}:status`, {
      playerId,
      status: 'online',
      zone: 'Ironforge',
      lastHeartbeat: now.toString(),
    })
    await client.expire(`game:presence:${playerId}:status`, 60)

    const isOnline = await client.sIsMember('game:presence:online_players', playerId)
    expect(isOnline).toBe(true)

    const status = await client.hGetAll(`game:presence:${playerId}:status`)
    expect(status.status).toBe('online')
    expect(status.zone).toBe('Ironforge')

    const ttl = await client.ttl(`game:presence:${playerId}:status`)
    expect(ttl).toBeGreaterThan(0)
  })

  // Test 2: Player logs out (goes offline)
  it('player logs out — goes offline', async () => {
    const playerId = 'player-1001'

    await client.sAdd('game:presence:online_players', playerId)
    await client.hSet(`game:presence:${playerId}:status`, 'status', 'online')

    await client
      .multi()
      .sRem('game:presence:online_players', playerId)
      .hSet(`game:presence:${playerId}:status`, 'status', 'offline')
      .expire(`game:presence:${playerId}:status`, 86400)
      .exec()

    const isOnline = await client.sIsMember('game:presence:online_players', playerId)
    expect(isOnline).toBe(false)

    const status = await client.hGet(`game:presence:${playerId}:status`, 'status')
    expect(status).toBe('offline')
  })

  // Test 3: Get all online players
  it('get all online players', async () => {
    await client.sAdd('game:presence:online_players', 'player-1001')
    await client.sAdd('game:presence:online_players', 'player-1002')
    await client.sAdd('game:presence:online_players', 'player-1003')

    const onlinePlayers = await client.sMembers('game:presence:online_players')
    expect(onlinePlayers.sort()).toEqual(['player-1001', 'player-1002', 'player-1003'])
  })

  // Test 4: Heartbeat renews TTL (player client sends periodic updates)
  it('heartbeat renews TTL — player sends periodic heartbeat', async () => {
    const playerId = 'player-1001'

    await client.hSet(`game:presence:${playerId}:status`, 'status', 'online')
    await client.expire(`game:presence:${playerId}:status`, 60)

    // Simulate heartbeat some time later
    await client.expire(`game:presence:${playerId}:status`, 60)

    const ttl = await client.ttl(`game:presence:${playerId}:status`)
    expect(ttl).toBeGreaterThan(55)
  })

  // Test 5: Check if player is online before sending message
  it('check if player is online before sending message', async () => {
    await client.sAdd('game:presence:online_players', 'player-1001')

    const isOnline = await client.sIsMember('game:presence:online_players', 'player-1001')
    const isOffline = await client.sIsMember('game:presence:online_players', 'player-9999')

    expect(isOnline).toBe(true)
    expect(isOffline).toBe(false)
  })
})