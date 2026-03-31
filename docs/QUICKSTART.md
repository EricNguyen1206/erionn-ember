# Quickstart Guide

**gomemkv** — in-memory key-value store written in Go, `redis-cli` compatible via RESP/TCP.

---

## 1. Run the Server

```bash
make build
./bin/gomemkv
# or: PORT=9090 ./bin/gomemkv
```

The server listens on `tcp://localhost:9090`.

Verify it's running:
```bash
redis-cli -p 9090 PING
# PONG
```

---

## 2. Basic Commands

```bash
redis-cli -p 9090 SET hello world
redis-cli -p 9090 GET hello          # "world"
redis-cli -p 9090 SET counter 0
redis-cli -p 9090 INCR counter       # (integer) 1
redis-cli -p 9090 DEL hello counter  # (integer) 2
```

---

## 3. Pub/Sub

**Terminal 1** — Subscribe:
```bash
redis-cli -p 9090 SUBSCRIBE my-channel
```

**Terminal 2** — Publish:
```bash
redis-cli -p 9090 PUBLISH my-channel "hello world"
# (integer) 1
```

Terminal 1 will display:
```
1) "message"
2) "my-channel"
3) "hello world"
```

---

## What's Next?

- [API Reference](API_REFERENCE.md) — all supported commands
- [Data Model](DATA_MODEL.md) — hashes, lists, sets
- [Pub/Sub Guide](PUB_SUB.md) — real-time messaging
- [Architecture](ARCHITECTURE.md) — internal design
