# API Reference

gomemkv uses the **RESP (Redis Serialization Protocol)** over TCP — fully compatible with `redis-cli` and any Redis client library.

Default port: `9090`

---

## Generic Commands

| Command | Args | Response |
|---|---|---|
| `PING [message]` | optional message | `+PONG` or bulk string |
| `DEL key [key ...]` | one or more keys | integer — number of keys deleted |
| `EXISTS key [key ...]` | one or more keys | integer — number of keys that exist |
| `TYPE key` | key | simple string: `string`, `hash`, `list`, `set`, `none` |
| `EXPIRE key seconds` | key, TTL in seconds | `1` if set, `0` if key not found |
| `TTL key` | key | seconds remaining, `-1` if no expiry, `-2` if not found |
| `INFO` | — | bulk string with server/keyspace/pubsub stats |
| `COMMAND` | — | `+OK` (minimal redis-cli handshake) |

---

## Strings

| Command | Args | Response |
|---|---|---|
| `SET key value [EX seconds] [PX millis]` | key, value, optional TTL | `+OK` |
| `GET key` | key | bulk string or nil |
| `INCR key` | key | integer — new value |

---

## Hashes

| Command | Args | Response |
|---|---|---|
| `HSET key field value [field value ...]` | key, field-value pairs | integer — number of new fields added |
| `HGET key field` | key, field | bulk string or nil |
| `HDEL key field [field ...]` | key, fields | integer — number of fields removed |
| `HGETALL key` | key | array — alternating field, value pairs |

---

## Lists

| Command | Args | Response |
|---|---|---|
| `LPUSH key value [value ...]` | key, values | integer — list length after push |
| `RPUSH key value [value ...]` | key, values | integer — list length after push |
| `LPOP key` | key | bulk string or nil |
| `RPOP key` | key | bulk string or nil |
| `LRANGE key start stop` | key, start, stop | array of strings (inclusive, supports negative indexes) |

---

## Sets

| Command | Args | Response |
|---|---|---|
| `SADD key member [member ...]` | key, members | integer — number of new members added |
| `SREM key member [member ...]` | key, members | integer — number of members removed |
| `SMEMBERS key` | key | array of strings (sorted) |
| `SISMEMBER key member` | key, member | `1` if member, `0` if not |
| `SCARD key` | key | integer — number of members |

---

## Pub/Sub

| Command | Args | Response |
|---|---|---|
| `PUBLISH channel payload` | channel, payload | integer — number of subscribers that received the message |
| `SUBSCRIBE channel [channel ...]` | channels | per-channel confirm: `*3 subscribe <channel> <count>` |
| `UNSUBSCRIBE [channel ...]` | optional channels (all if omitted) | per-channel confirm: `*3 unsubscribe <channel> <count>` |

**Subscription mode**: once subscribed, only `SUBSCRIBE`, `UNSUBSCRIBE`, and `PING` are allowed.

**Message push format**: `*3\r\n$7\r\nmessage\r\n$<len>\r\n<channel>\r\n$<len>\r\n<payload>\r\n`

---

## Error Responses

- Wrong type for key → `-WRONGTYPE Operation against a key holding the wrong kind of value`
- Missing/bad args → `-ERR wrong number of arguments for '<cmd>' command`
- Unknown command → `-ERR unknown command '<cmd>'`
- Missing data is returned as nil (`$-1\r\n`) rather than an error
