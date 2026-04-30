const { createClient } = require('redis');
async function run() {
  const client = createClient({ url: 'redis://localhost:9090' });
  await client.connect();
  await client.set('a', '1');
  await client.set('b', '2');
  const deleted = await client.del('a', 'b');
  console.log('Deleted:', deleted);
  const existsB = await client.exists('b');
  console.log('Exists b:', existsB);
  await client.quit();
}
run().catch(console.error);
