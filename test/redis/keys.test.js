// Covers: EXISTS, EXPIRE, TTL, TYPE, DEL
// Context: Online Game — temporary buffs, session tokens, world object types
const { createRedisClient } = require('./redis')

describe('Keys — EXPIRE / TTL / EXISTS / TYPE / DEL', () => {
  let client

  beforeAll(async () => {
    client = await createRedisClient()
  })

  afterAll(async () => {
    await client.quit()
  })

  afterEach(async () => {
    await client.del([
      'game:key:session:1001',
      'game:key:buff:1001',
      'game:key:guild:1',
      'game:key:online_players',
      'game:key:config',
    ])
  })

  // Test 1: EXISTS returns 0 for non-existent key
  it('EXISTS returns 0 for non-existent key', async () => {
    const exists = await client.exists('game:key:nonexistent')
    expect(exists).toBe(0)
  })

  // Test 2: EXISTS returns 1 for existing key
  it('EXISTS returns 1 for existing key', async () => {
    await client.set('game:key:config', 'v1.0.0')
    const exists = await client.exists('game:key:config')
    expect(exists).toBe(1)
  })

  // Test 3: EXPIRE sets TTL on key
  it('EXPIRE sets TTL — temporary power-up buff', async () => {
    await client.set('game:key:buff:1001', 'double_xp')
    const ok = await client.expire('game:key:buff:1001', 3600)
    expect(ok).toBe(true)
    const ttl = await client.ttl('game:key:buff:1001')
    expect(ttl).toBeGreaterThan(0)
    expect(ttl).toBeLessThanOrEqual(3600)
  })

  // Test 4: TTL returns -1 for key without expiry
  it('TTL returns -1 for key without expiry', async () => {
    await client.set('game:key:config', 'v1.0.0')
    const ttl = await client.ttl('game:key:config')
    expect(ttl).toBe(-1)
  })

  // Test 5: TTL returns -2 for non-existent key
  it('TTL returns -2 for non-existent key', async () => {
    const ttl = await client.ttl('game:key:nonexistent')
    expect(ttl).toBe(-2)
  })

  // Test 6: TYPE returns "string" for string type
  it('TYPE returns correct type for string — game version', async () => {
    await client.set('game:key:config', 'v1.0.0')
    const type = await client.type('game:key:config')
    expect(type).toBe('string')
  })

  // Test 7: TYPE returns "hash" for hash type
  it('TYPE returns correct type for hash — player profile', async () => {
    await client.hSet('game:key:player:1001', 'username', 'ShadowWalker')
    const type = await client.type('game:key:player:1001')
    expect(type).toBe('hash')
    await client.del('game:key:player:1001')
  })

  // Test 8: TYPE returns "list" for list type
  it('TYPE returns correct type for list — global chat', async () => {
    await client.lPush('game:key:chat:global', 'Hello world!')
    const type = await client.type('game:key:chat:global')
    expect(type).toBe('list')
    await client.del('game:key:chat:global')
  })

  // Test 9: TYPE returns "set" for set type
  it('TYPE returns correct type for set — online players', async () => {
    await client.sAdd('game:key:online_players', 'player-1001')
    const type = await client.type('game:key:online_players')
    expect(type).toBe('set')
  })

  // Test 10: TYPE returns "none" for non-existent key
  it('TYPE returns none for non-existent key', async () => {
    const type = await client.type('game:key:nonexistent')
    expect(type).toBe('none')
  })

  // Test 11: DEL removes key and EXISTS confirms removal
  it('DEL removes key and EXISTS confirms — end session', async () => {
    await client.set('game:key:session:1001', 'token_123')
    expect(await client.exists('game:key:session:1001')).toBe(1)
    await client.del('game:key:session:1001')
    expect(await client.exists('game:key:session:1001')).toBe(0)
  })
})