// Sets — erion-raven uses: SADD, SREM, SMEMBERS, SISMEMBER
// Also covers: SCARD (used in erion-raven conversation participant count)
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
      'test:set:online_users',
      'test:set:conv:1:members',
      'test:set:user:1:conversations'
    ])
  })

  it('SADD + SMEMBERS basic (erion-raven online users pattern)', async () => {
    await client.sAdd('test:set:online_users', 'user-1')
    await client.sAdd('test:set:online_users', 'user-2')
    const members = await client.sMembers('test:set:online_users')
    expect(members.sort()).toEqual(['user-1', 'user-2'])
  })

  it('SADD ignores duplicates', async () => {
    await client.sAdd('test:set:online_users', 'user-1')
    const added = await client.sAdd('test:set:online_users', 'user-1')
    expect(added).toBe(0)
  })

  it('SISMEMBER checks membership (erion-raven presence check)', async () => {
    await client.sAdd('test:set:online_users', 'user-1')
    const isOnline = await client.sIsMember('test:set:online_users', 'user-1')
    expect(isOnline).toBe(true)
    const isOffline = await client.sIsMember('test:set:online_users', 'user-99')
    expect(isOffline).toBe(false)
  })

  it('SREM removes member (erion-raven offline pattern)', async () => {
    await client.sAdd('test:set:online_users', 'user-1')
    await client.sAdd('test:set:online_users', 'user-2')
    await client.sRem('test:set:online_users', 'user-1')
    const members = await client.sMembers('test:set:online_users')
    expect(members).toEqual(['user-2'])
  })

  it('SMEMBERS returns empty for non-existent key', async () => {
    const members = await client.sMembers('test:set:nonexistent')
    expect(members).toEqual([])
  })

  it('SCARD returns set size (erion-raven participant count)', async () => {
    await client.sAdd('test:set:conv:1:members', 'user-1')
    await client.sAdd('test:set:conv:1:members', 'user-2')
    await client.sAdd('test:set:conv:1:members', 'user-3')
    const count = await client.sCard('test:set:conv:1:members')
    expect(count).toBe(3)
  })

  it('Bidirectional link pattern (erion-raven conversation membership)', async () => {
    // conversation → members
    await client.sAdd('test:set:conv:1:members', 'user-1')
    // user → conversations
    await client.sAdd('test:set:user:1:conversations', 'conv-1')

    const convMembers = await client.sMembers('test:set:conv:1:members')
    expect(convMembers).toEqual(['user-1'])

    const userConvs = await client.sMembers('test:set:user:1:conversations')
    expect(userConvs).toEqual(['conv-1'])
  })
})
