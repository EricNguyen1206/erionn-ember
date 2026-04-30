// Strings — erion-raven uses: SET, GET, SETEX, DEL
// Also covers: INCR (supported by gomemkv)
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
    await client.del(['test:str:1', 'test:str:2', 'test:str:cache'])
  })

  it('SET + GET basic string', async () => {
    await client.set('test:str:1', 'hello')
    const val = await client.get('test:str:1')
    expect(val).toBe('hello')
  })

  it('SET overwrites existing value', async () => {
    await client.set('test:str:1', 'first')
    await client.set('test:str:1', 'second')
    const val = await client.get('test:str:1')
    expect(val).toBe('second')
  })

  it('GET returns null for non-existent key', async () => {
    const val = await client.get('test:str:nonexistent')
    expect(val).toBeNull()
  })

  it('DEL removes a key', async () => {
    await client.set('test:str:1', 'bye')
    await client.del('test:str:1')
    const val = await client.get('test:str:1')
    expect(val).toBeNull()
  })

  it('DEL multiple keys at once (erion-raven cache pattern)', async () => {
    await client.set('test:str:1', 'a')
    await client.set('test:str:2', 'b')
    const deleted = await client.del(['test:str:1', 'test:str:2'])
    expect(deleted).toBe(2)
  })

  it('SET + GET JSON string (erion-raven cache pattern)', async () => {
    const data = { id: 1, name: 'test' }
    await client.set('test:str:cache', JSON.stringify(data))
    const raw = await client.get('test:str:cache')
    expect(JSON.parse(raw)).toEqual(data)
  })

  it('SETEX sets key with TTL (erion-raven cache pattern)', async () => {
    await client.setEx('test:str:cache', 60, JSON.stringify({ cached: true }))
    const raw = await client.get('test:str:cache')
    expect(JSON.parse(raw)).toEqual({ cached: true })
    const ttl = await client.ttl('test:str:cache')
    expect(ttl).toBeGreaterThan(0)
    expect(ttl).toBeLessThanOrEqual(60)
  })

  it('INCR increments numeric value', async () => {
    await client.set('test:str:1', '10')
    const result = await client.incr('test:str:1')
    expect(result).toBe(11)
    const val = await client.get('test:str:1')
    expect(val).toBe('11')
  })

  it('INCR starts from 0 for non-existent key', async () => {
    const result = await client.incr('test:str:counter_new')
    expect(result).toBe(1)
    await client.del('test:str:counter_new')
  })
})
