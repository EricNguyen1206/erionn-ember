// Covers: SADD, SREM, SMEMBERS, SISMEMBER, SCARD
// Context: Online Game — guild members, inventory, achievements, friend lists
const { createRedisClient } = require('./redis')

describe('Sets — SADD / SREM / SMEMBERS / SISMEMBER / SCARD', () => {
  let client

  beforeAll(async () => {
    client = await createRedisClient()
  })

  afterAll(async () => {
    await client.quit()
  })

  afterEach(async () => {
    await client.del([
      'guild:dragon_slayers:members',
      'player:1001:inventory',
      'game:achievements:rare',
      'player:1001:friends',
      'player:1002:friends',
    ])
  })

  // Test 1: SADD + SMEMBERS basic operations
  it('SADD + SMEMBERS basic — add members to a guild', async () => {
    await client.sAdd('guild:dragon_slayers:members', 'ShadowWalker')
    await client.sAdd('guild:dragon_slayers:members', 'LightBringer')
    await client.sAdd('guild:dragon_slayers:members', 'FireMage')
    const members = await client.sMembers('guild:dragon_slayers:members')
    expect(members.sort()).toEqual(['FireMage', 'LightBringer', 'ShadowWalker'])
  })

  // Test 2: SADD ignores duplicates
  it('SADD ignores duplicates — no duplicate guild members', async () => {
    await client.sAdd('guild:dragon_slayers:members', 'ShadowWalker')
    const added = await client.sAdd('guild:dragon_slayers:members', 'ShadowWalker')
    expect(added).toBe(0)
  })

  // Test 3: SISMEMBER checks membership
  it('SISMEMBER checks membership — check if player has an item', async () => {
    await client.sAdd('player:1001:inventory', 'excalibur')
    const hasExcalibur = await client.sIsMember('player:1001:inventory', 'excalibur')
    expect(hasExcalibur).toBe(true)
    const hasShield = await client.sIsMember('player:1001:inventory', 'wooden_shield')
    expect(hasShield).toBe(false)
  })

  // Test 4: SREM removes member
  it('SREM removes member — remove item from inventory', async () => {
    await client.sAdd('player:1001:inventory', 'potion_hp')
    await client.sAdd('player:1001:inventory', 'potion_mp')
    await client.sRem('player:1001:inventory', 'potion_hp')
    const members = await client.sMembers('player:1001:inventory')
    expect(members).toEqual(['potion_mp'])
  })

  // Test 5: SMEMBERS returns empty for non-existent key
  it('SMEMBERS returns empty for non-existent key', async () => {
    const members = await client.sMembers('player:1001:inventory:nonexistent')
    expect(members).toEqual([])
  })

  // Test 6: SCARD returns set size
  it('SCARD returns set size — count rare achievements', async () => {
    await client.sAdd('game:achievements:rare', 'DragonSlayer')
    await client.sAdd('game:achievements:rare', 'WorldFirst')
    await client.sAdd('game:achievements:rare', 'LegendaryHero')
    const count = await client.sCard('game:achievements:rare')
    expect(count).toBe(3)
  })

  // Test 7: Bidirectional link pattern
  it('Bidirectional link — player friend lists', async () => {
    await client.sAdd('player:1001:friends', 'player-1002')
    await client.sAdd('player:1002:friends', 'player-1001')

    const friends1001 = await client.sMembers('player:1001:friends')
    expect(friends1001).toEqual(['player-1002'])

    const friends1002 = await client.sMembers('player:1002:friends')
    expect(friends1002).toEqual(['player-1001'])
  })
})