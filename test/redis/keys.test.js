// Keys — erion-raven uses: EXPIRE, DEL
// Also covers: EXISTS, TYPE, TTL (supported by gomemkv)
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
    await client.del(['test:key:1', 'test:key:2'])
  })

  it('EXISTS returns false for non-existent key', async () => {
    const exists = await client.exists('test:key:nonexistent')
    expect(exists).toBe(0)
  })

  it('EXISTS returns true for existing key', async () => {
    await client.set('test:key:1', 'value')
    const exists = await client.exists('test:key:1')
    expect(exists).toBe(1)
  })

  it('EXPIRE sets TTL (erion-raven status TTL pattern)', async () => {
    await client.set('test:key:1', 'value')
    const ok = await client.expire('test:key:1', 60)
    expect(ok).toBe(true)
    const ttl = await client.ttl('test:key:1')
    expect(ttl).toBeGreaterThan(0)
    expect(ttl).toBeLessThanOrEqual(60)
  })

  it('TTL returns -1 for key without expiry', async () => {
    await client.set('test:key:1', 'value')
    const ttl = await client.ttl('test:key:1')
    expect(ttl).toBe(-1)
  })

  it('TTL returns -2 for non-existent key', async () => {
    const ttl = await client.ttl('test:key:nonexistent')
    expect(ttl).toBe(-2)
  })

  it('TYPE returns correct type for string', async () => {
    await client.set('test:key:1', 'value')
    const type = await client.type('test:key:1')
    expect(type).toBe('string')
  })

  it('TYPE returns correct type for hash', async () => {
    await client.hSet('test:key:1', 'field', 'value')
    const type = await client.type('test:key:1')
    expect(type).toBe('hash')
  })

  it('TYPE returns correct type for list', async () => {
    await client.lPush('test:key:1', 'value')
    const type = await client.type('test:key:1')
    expect(type).toBe('list')
  })

  it('TYPE returns correct type for set', async () => {
    await client.sAdd('test:key:1', 'member')
    const type = await client.type('test:key:1')
    expect(type).toBe('set')
  })

  it('TYPE returns none for non-existent key', async () => {
    const type = await client.type('test:key:nonexistent')
    expect(type).toBe('none')
  })

  it('DEL removes key and EXISTS confirms', async () => {
    await client.set('test:key:1', 'value')
    expect(await client.exists('test:key:1')).toBe(1)
    await client.del('test:key:1')
    expect(await client.exists('test:key:1')).toBe(0)
  })
})
