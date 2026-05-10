// Covers: MULTI, EXEC (atomic operations)
// Context: Online Game — atomic item purchase, quest completion, trading
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
      'game:tx:online_players',
      'game:tx:player:1001:profile',
      'game:tx:player:1001:inventory',
      'game:tx:quest:501:status',
    ])
  })

  // Test 1: MULTI/EXEC atomic set operations
  it('MULTI/EXEC atomic set — player login + add to online set', async () => {
    const results = await client
      .multi()
      .set('game:tx:player:1001:last_login', new Date().toISOString())
      .sAdd('game:tx:online_players', 'player-1001')
      .exec()

    expect(results).toHaveLength(2)
    expect(results[0]).toEqual(expect.stringMatching(/OK/i))
    expect(results[1]).toBe(1)
    await client.del('game:tx:player:1001:last_login')
  })

  // Test 2: MULTI/EXEC atomic hash + set operations
  it('MULTI/EXEC atomic hash + set — purchase item (deduct gold + add to inventory)', async () => {
    // Initial state
    await client.hSet('game:tx:player:1001:profile', 'gold', '1000')

    const results = await client
      .multi()
      .hSet('game:tx:player:1001:profile', 'gold', '800') // Deduct 200 gold
      .sAdd('game:tx:player:1001:inventory', 'steel_sword') // Add item
      .expire('game:tx:player:1001:inventory', 86400)
      .exec()

    expect(results).toHaveLength(3)

    const gold = await client.hGet('game:tx:player:1001:profile', 'gold')
    expect(gold).toBe('800')

    const inventory = await client.sMembers('game:tx:player:1001:inventory')
    expect(inventory).toContain('steel_sword')
  })

  // Test 3: MULTI/EXEC update quest status atomically
  it('MULTI/EXEC update quest status — quest completed -> reward given', async () => {
    await client.hSet('game:tx:quest:501:status', 'state', 'active')
    await client.hSet('game:tx:player:1001:profile', 'xp', '5000')

    await client
      .multi()
      .hSet('game:tx:quest:501:status', 'state', 'completed')
      .hSet('game:tx:player:1001:profile', 'xp', '5500') // Reward 500 XP
      .sAdd('game:tx:player:1001:achievements', 'QuestMaster')
      .exec()

    const questState = await client.hGet('game:tx:quest:501:status', 'state')
    expect(questState).toBe('completed')

    const xp = await client.hGet('game:tx:player:1001:profile', 'xp')
    expect(xp).toBe('5500')

    await client.del('game:tx:player:1001:achievements')
  })

  // Test 4: MULTI/EXEC processes multiple player updates atomically
  it('MULTI/EXEC process multiple player updates atomically', async () => {
    await client
      .multi()
      .sAdd('game:tx:online_players', 'player-1001')
      .sAdd('game:tx:online_players', 'player-1002')
      .hSet('game:tx:player:1001:profile', 'status', 'in-dungeon')
      .hSet('game:tx:player:1002:profile', 'status', 'in-dungeon')
      .exec()

    const onlinePlayers = await client.sMembers('game:tx:online_players')
    expect(onlinePlayers.sort()).toEqual(['player-1001', 'player-1002'])

    await client.del('game:tx:player:1002:profile')
  })
})