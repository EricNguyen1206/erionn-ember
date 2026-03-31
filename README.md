# gomemkv

In-memory key-value store (Redis-like) with RESP/TCP protocol and pub/sub. Written in Go.

has:
- strings
- hashes
- lists
- sets
- pubsub

does not have:
- persistence
- auth
- clustering

ports:
- tcp `9090` (RESP protocol, `redis-cli` compatible)

run:

```bash
make build
./bin/gomemkv
```

dev:

```bash
make test
make test-race
make lint
```

more:
- `docs/API_REFERENCE.md`
- `docs/ARCHITECTURE.md`
