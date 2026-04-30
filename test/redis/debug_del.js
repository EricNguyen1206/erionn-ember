const { createClient } = require('redis');
async function run() {
  const client = createClient({ socket: { port: 9090 } });
  await client.connect();
  await client.sAdd('set1', 'a');
  await client.sAdd('set2', 'b');
  console.log('sets created');
  const res = await client.del(['set1', 'set2']);
  console.log('del res:', res);
  const m1 = await client.sMembers('set1');
  const m2 = await client.sMembers('set2');
  console.log('m1:', m1, 'm2:', m2);
  await client.quit();
}
run().catch(console.error);
