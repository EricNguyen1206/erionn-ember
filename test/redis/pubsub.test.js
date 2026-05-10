// Covers: PUBLISH, SUBSCRIBE, UNSUBSCRIBE
// Context: Online Game — world boss spawn alerts, global system announcements, private messages
const { createClient } = require('redis')
const { REDIS_URL } = require('./redis')

describe('Pub/Sub — PUBLISH / SUBSCRIBE', () => {
  let publisher, subscriber

  beforeAll(async () => {
    publisher = createClient({ url: REDIS_URL })
    subscriber = createClient({ url: REDIS_URL })
    publisher.on('error', () => {})
    subscriber.on('error', () => {})
    await publisher.connect()
    await subscriber.connect()
  })

  afterAll(async () => {
    await publisher.quit()
    await subscriber.quit()
  })

  // Test 1: PUBLISH delivers message to subscriber
  it('PUBLISH delivers message — notify world of boss spawn', async () => {
    const channel = 'game:pubsub:world_events'
    const message = JSON.stringify({
      event: 'BOSS_SPAWN',
      bossName: 'Ancient Dragon',
      location: 'Crystal Peak',
      level: 99,
    })

    const received = new Promise((resolve) => {
      subscriber.subscribe(channel, (msg) => {
        resolve(msg)
      })
    })

    await new Promise((r) => setTimeout(r, 100))
    await publisher.publish(channel, message)

    const result = await received
    expect(result).toBe(message)
    await subscriber.unsubscribe(channel)
  })

  // Test 2: PUBLISH to private player notification channel
  it('PUBLISH to player notification — trade request received', async () => {
    const channel = 'game:pubsub:player:1001:notifications'
    const message = JSON.stringify({
      type: 'trade_request',
      from: 'ShadowWalker',
      itemId: 'sword_001',
      expiresIn: 30,
    })

    const received = new Promise((resolve) => {
      subscriber.subscribe(channel, (msg) => {
        resolve(msg)
      })
    })

    await new Promise((r) => setTimeout(r, 100))
    await publisher.publish(channel, message)

    const result = await received
    expect(JSON.parse(result)).toEqual({
      type: 'trade_request',
      from: 'ShadowWalker',
      itemId: 'sword_001',
      expiresIn: 30,
    })
    await subscriber.unsubscribe(channel)
  })

  // Test 3: PUBLISH returns subscriber count
  it('PUBLISH returns subscriber count — multiple players receive global announcement', async () => {
    const channel = 'game:pubsub:global_announcements'
    await subscriber.subscribe(channel, () => {})
    await new Promise((r) => setTimeout(r, 100))

    const receivers = await publisher.publish(channel, 'Server maintenance in 30 minutes!')
    expect(receivers).toBeGreaterThanOrEqual(1)
    await subscriber.unsubscribe(channel)
  })
})