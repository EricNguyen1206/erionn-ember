// Covers: HSET, HGET, HGETALL, HDEL
// Context: Online Game — player profiles, item stats, guild info
const { createRedisClient } = require('./redis')

describe('Hashes — HSET / HGET / HGETALL / HDEL', () => {
  let client

  beforeAll(async () => {
    client = await createRedisClient()
  })

  afterAll(async () => {
    await client.quit()
  })

  afterEach(async () => {
    await client.del([
      'player:1001:profile',
      'item:2001:stats',
      'guild:dragon_slayers:info',
    ])
  })

  // Test 1: HSET + HGET single field
  it('HSET + HGET single field — store player level', async () => {
    await client.hSet('player:1001:profile', 'level', '42')
    const level = await client.hGet('player:1001:profile', 'level')
    expect(level).toBe('42')
  })

  // Test 2: HSET multiple fields at once
  it('HSET multiple fields — store full player profile', async () => {
    await client.hSet('player:1001:profile', {
      username: 'ShadowWalker',
      level: '42',
      class: 'Assassin',
      gold: '1500',
    })
    const username = await client.hGet('player:1001:profile', 'username')
    expect(username).toBe('ShadowWalker')
    const gold = await client.hGet('player:1001:profile', 'gold')
    expect(gold).toBe('1500')
  })

  // Test 3: HGET returns null for non-existent field
  it('HGET returns null for non-existent field', async () => {
    await client.hSet('player:1001:profile', 'username', 'ShadowWalker')
    const val = await client.hGet('player:1001:profile', 'achievements')
    expect(val).toBeNull()
  })

  // Test 4: HGETALL returns all fields
  it('HGETALL returns all fields — view item stats', async () => {
    await client.hSet('item:2001:stats', {
      name: 'Excalibur',
      type: 'Sword',
      damage: '150',
      rarity: 'Legendary',
      durability: '100',
    })
    const all = await client.hGetAll('item:2001:stats')
    expect(all).toEqual({
      name: 'Excalibur',
      type: 'Sword',
      damage: '150',
      rarity: 'Legendary',
      durability: '100',
    })
  })

  // Test 5: HGETALL returns empty object for non-existent key
  it('HGETALL returns empty object for non-existent key', async () => {
    const all = await client.hGetAll('player:9999:profile')
    expect(all).toEqual({})
  })

  // Test 6: HDEL removes specific field
  it('HDEL removes specific field — remove player temporary title', async () => {
    await client.hSet('player:1001:profile', {
      username: 'ShadowWalker',
      title: 'Dragon Slayer',
      level: '42',
    })
    await client.hDel('player:1001:profile', 'title')
    const remaining = await client.hGetAll('player:1001:profile')
    expect(remaining).toEqual({
      username: 'ShadowWalker',
      level: '42',
    })
  })

  // Test 7: HSET overwrites existing field
  it('HSET overwrites existing field — update player gold after purchase', async () => {
    await client.hSet('player:1001:profile', 'gold', '1500')
    await client.hSet('player:1001:profile', 'gold', '1200')
    const gold = await client.hGet('player:1001:profile', 'gold')
    expect(gold).toBe('1200')
  })
})