# gomemkv

In-memory key-value store (Redis-like) with RESP/TCP protocol and pub/sub. Written in Go.

**has:**
- strings, hashes, lists, sets
- pub/sub

**does not have:**
- persistence
- auth
- clustering

**port:** tcp `9090` (RESP protocol, `redis-cli` compatible)

## Run

```bash
go build -o bin/gomemkv ./cmd/gomemkv
./bin/gomemkv
```

Test with `redis-cli`:

```bash
redis-cli -p 9090
127.0.0.1:9090> SET greeting "Hello gomemkv"
OK
127.0.0.1:9090> GET greeting
"Hello gomemkv"
```

## Dev

```bash
go test ./...
go test -race ./...
```

## Docs

Full documentation is available on the [wiki](https://github.com/EricNguyen1206/gomemkv/wiki).
