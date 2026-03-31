# Pub/Sub Guide

gomemkv supports Redis-compatible pub/sub over TCP. Any Redis client or `redis-cli` works out of the box.

## How it Works

1. **Subscriber** connects and sends `SUBSCRIBE channel [channel ...]` — enters subscription mode
2. **Publisher** calls `PUBLISH channel payload` from any connection
3. The internal `Hub` fans out the message to all active subscribers on that channel
4. If a subscriber's buffer is full (slow consumer), it is automatically removed

Channels do not need to be created in advance — they appear when the first subscriber joins and disappear when the last one leaves.

---

## Subscribing

```bash
redis-cli -p 9090 SUBSCRIBE system-events notifications
```

Response per channel:
```
1) "subscribe"
2) "system-events"
3) (integer) 1
```

Incoming messages look like:
```
1) "message"
2) "system-events"
3) "Server restarted"
```

**Subscription mode rules**: while subscribed, only `SUBSCRIBE`, `UNSUBSCRIBE`, and `PING` are accepted. All other commands return an error.

---

## Unsubscribing

```bash
# Unsubscribe from a specific channel
redis-cli -p 9090 UNSUBSCRIBE system-events

# Or Ctrl+C to disconnect (subscriber is automatically cleaned up)
```

---

## Publishing

```bash
redis-cli -p 9090 PUBLISH system-events "Server restarted"
# (integer) 1  ← number of subscribers that received the message
```

If no subscribers are active, the message is dropped (not persisted).

---

## Go Client Example

```go
import "github.com/redis/go-redis/v9"

// Subscriber
rdb := redis.NewClient(&redis.Options{Addr: "localhost:9090"})
sub := rdb.Subscribe(ctx, "my-channel")
defer sub.Close()

ch := sub.Channel()
for msg := range ch {
    fmt.Printf("[%s] %s\n", msg.Channel, msg.Payload)
}

// Publisher (separate connection)
rdb.Publish(ctx, "my-channel", "hello world")
```

---

## Notes

- Messages are not persisted — fire and forget
- Slow subscribers are dropped to protect server memory
- The `Hub` is fully independent from the key-value `Store`
