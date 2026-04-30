const { createClient } = require('redis')

const REDIS_URL = process.env.REDIS_URL || 'redis://localhost:9090'

async function createRedisClient() {
  const client = createClient({ url: REDIS_URL })

  client.on('error', (err) => {
    console.error('Redis client error:', err)
  })

  await client.connect()
  return client
}

module.exports = { createRedisClient, REDIS_URL }
