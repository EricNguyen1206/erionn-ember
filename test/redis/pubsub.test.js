// Pub/Sub — erion-raven uses: PUBLISH for real-time chat events
// chat:conversation:*, conversation:*:events, user:*:notifications
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

  it('PUBLISH delivers message to subscriber (erion-raven chat pattern)', async () => {
    const channel = 'test:pubsub:chat:conv:1'
    const message = JSON.stringify({ from: 'user-1', text: 'hello' })

    const received = new Promise((resolve) => {
      subscriber.subscribe(channel, (msg) => {
        resolve(msg)
      })
    })

    // Small delay to ensure subscription is active
    await new Promise((r) => setTimeout(r, 100))
    await publisher.publish(channel, message)

    const result = await received
    expect(result).toBe(message)
    await subscriber.unsubscribe(channel)
  })

  it('PUBLISH to user notification channel (erion-raven notification pattern)', async () => {
    const channel = 'test:pubsub:user:user-1:notifications'
    const message = JSON.stringify({ type: 'friend_online', userId: 'user-2' })

    const received = new Promise((resolve) => {
      subscriber.subscribe(channel, (msg) => {
        resolve(msg)
      })
    })

    await new Promise((r) => setTimeout(r, 100))
    await publisher.publish(channel, message)

    const result = await received
    expect(JSON.parse(result)).toEqual({ type: 'friend_online', userId: 'user-2' })
    await subscriber.unsubscribe(channel)
  })

  it('PUBLISH returns subscriber count', async () => {
    const channel = 'test:pubsub:count'
    await subscriber.subscribe(channel, () => {})
    await new Promise((r) => setTimeout(r, 100))

    const receivers = await publisher.publish(channel, 'test')
    expect(receivers).toBeGreaterThanOrEqual(1)
    await subscriber.unsubscribe(channel)
  })
})
