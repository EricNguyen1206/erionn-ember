// Hashes — erion-raven uses: HSET, HGET, HGETALL
// Also covers: HDEL (supported by gomemkv)
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
    await client.del(['test:hash:user:1', 'test:hash:migration'])
  })

  it('HSET + HGET single field (erion-raven user status pattern)', async () => {
    await client.hSet('test:hash:user:1', 'status', 'online')
    const status = await client.hGet('test:hash:user:1', 'status')
    expect(status).toBe('online')
  })

  it('HSET multiple fields at once (erion-raven user status pattern)', async () => {
    await client.hSet('test:hash:user:1', {
      userId: 'user-1',
      status: 'online',
      lastSeen: Date.now().toString(),
    })
    const status = await client.hGet('test:hash:user:1', 'status')
    expect(status).toBe('online')
    const userId = await client.hGet('test:hash:user:1', 'userId')
    expect(userId).toBe('user-1')
  })

  it('HGET returns undefined for non-existent field', async () => {
    await client.hSet('test:hash:user:1', 'status', 'online')
    const val = await client.hGet('test:hash:user:1', 'nonexistent')
    expect(val).toBeNull()
  })

  it('HGETALL returns all fields (erion-raven migration status pattern)', async () => {
    await client.hSet('test:hash:migration', {
      version: '3',
      status: 'completed',
      timestamp: '2024-01-01',
    })
    const all = await client.hGetAll('test:hash:migration')
    expect(all).toEqual({
      version: '3',
      status: 'completed',
      timestamp: '2024-01-01',
    })
  })

  it('HGETALL returns empty object for non-existent key', async () => {
    const all = await client.hGetAll('test:hash:nonexistent')
    expect(all).toEqual({})
  })

  it('HDEL removes specific field', async () => {
    await client.hSet('test:hash:user:1', {
      status: 'online',
      lastSeen: 'now',
    })
    await client.hDel('test:hash:user:1', 'lastSeen')
    const remaining = await client.hGetAll('test:hash:user:1')
    expect(remaining).toEqual({ status: 'online' })
  })

  it('HSET overwrites existing field', async () => {
    await client.hSet('test:hash:user:1', 'status', 'online')
    await client.hSet('test:hash:user:1', 'status', 'offline')
    const status = await client.hGet('test:hash:user:1', 'status')
    expect(status).toBe('offline')
  })
})
