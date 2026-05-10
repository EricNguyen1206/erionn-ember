// Covers: SET, GET, SETEX, DEL, INCR
// Context: Online Game — game config, MOTD, player session tokens, world stats
const { createRedisClient } = require('./redis')

describe('Strings — SET / GET / SETEX / DEL / INCR', () => {
  let client

  beforeAll(async () => {
    client = await createRedisClient()
  })

  afterAll(async () => {
    await client.quit()
  })

  afterEach(async () => {
    await client.del([
      'game:str:motd',
      'game:str:maintenance',
      'game:str:session:1001',
      'game:str:total_players_joined',
    ])
  })

  // Test 1: SET + GET basic string
  it('SET + GET basic string — store MOTD', async () => {
    await client.set('game:str:motd', 'Welcome to Gomemkv Online! Enjoy your adventure.')
    const val = await client.get('game:str:motd')
    expect(val).toBe('Welcome to Gomemkv Online! Enjoy your adventure.')
  })

  // Test 2: SET overwrites existing value
  it('SET overwrites existing value — update maintenance message', async () => {
    await client.set('game:str:maintenance', 'Server will be down at 10 PM')
    await client.set('game:str:maintenance', 'Maintenance postponed to 11 PM')
    const val = await client.get('game:str:maintenance')
    expect(val).toBe('Maintenance postponed to 11 PM')
  })

  // Test 3: GET returns null for non-existent key
  it('GET returns null for non-existent key', async () => {
    const val = await client.get('game:str:nonexistent')
    expect(val).toBeNull()
  })

  // Test 4: DEL removes a single key
  it('DEL removes a key — remove session token', async () => {
    await client.set('game:str:session:1001', 'token_abcd_1234')
    await client.del('game:str:session:1001')
    const val = await client.get('game:str:session:1001')
    expect(val).toBeNull()
  })

  // Test 5: DEL removes multiple keys at once
  it('DEL multiple keys — clear multiple sessions', async () => {
    await client.set('game:str:session:1001', 'token_1')
    await client.set('game:str:session:1002', 'token_2')
    const deleted = await client.del(['game:str:session:1001', 'game:str:session:1002'])
    expect(deleted).toBe(2)
  })

  // Test 6: SET + GET JSON string (config pattern)
  it('SET + GET JSON string — store game world config', async () => {
    const config = { version: '1.2.3', maxPlayers: 1000, pvpEnabled: true, doubleXP: false }
    await client.set('game:str:config', JSON.stringify(config))
    const raw = await client.get('game:str:config')
    expect(JSON.parse(raw)).toEqual(config)
    await client.del('game:str:config')
  })

  // Test 7: SETEX with TTL (temporary buff)
  it('SETEX sets key with TTL — store temporary player buff for 60s', async () => {
    const buff = { type: 'StrengthBoost', multiplier: 1.5, duration: 60 }
    await client.setEx('player:1001:buff:str', 60, JSON.stringify(buff))
    const raw = await client.get('player:1001:buff:str')
    expect(JSON.parse(raw)).toEqual(buff)
    const ttl = await client.ttl('player:1001:buff:str')
    expect(ttl).toBeGreaterThan(0)
    expect(ttl).toBeLessThanOrEqual(60)
    await client.del('player:1001:buff:str')
  })

  // Test 8: INCR increments numeric value
  it('INCR increments numeric value — count total players joined', async () => {
    await client.set('game:str:total_players_joined', '9999')
    const result = await client.incr('game:str:total_players_joined')
    expect(result).toBe(10000)
    const val = await client.get('game:str:total_players_joined')
    expect(val).toBe('10000')
  })

  // Test 9: INCR starts from 0 for non-existent key
  it('INCR starts from 0 — first player joins today', async () => {
    const result = await client.incr('game:str:total_players_joined')
    expect(result).toBe(1)
    await client.del('game:str:total_players_joined')
  })
})