# Erion Ember

In-memory data cache in Go.

Uses gRPC for data commands and HTTP only for `/health`, `/ready`, and `/metrics`.

## Scope

- Single-node only
- Memory-only runtime
- Data is lost on restart
- No replication, clustering, persistence, transactions, or Lua scripting
- Pub/sub is realtime only; offline subscribers do not receive old messages

## Build

```bash
make build
```

Or directly:

```bash
go build -o bin/erion-ember ./cmd/server/
```

## Run

```bash
./bin/erion-ember
```

Default ports:

- HTTP admin: `8080`
- gRPC data API: `9090`

## Run

Start the server:

```bash
make build
./bin/erion-ember
```

Check admin endpoints:

```bash
curl http://localhost:8080/health
curl http://localhost:8080/ready
curl http://localhost:8080/metrics
```

Run the Go examples:

```bash
go run ./examples/go/raw-grpc-cache
go run ./examples/go/raw-grpc-pubsub
```

## gRPC API

Service: `ember.v1.CacheService`

Command groups:

- Generic: `Del`, `Exists`, `Type`, `Expire`, `Ttl`, `Stats`, `Health`
- Strings: `Get`, `Set`
- Hashes: `HSet`, `HGet`, `HDel`, `HGetAll`
- Lists: `LPush`, `RPush`, `LPop`, `RPop`, `LRange`
- Sets: `SAdd`, `SRem`, `SMembers`, `SIsMember`
- Pub/Sub: `Publish`, `Subscribe`

See `docs/API_REFERENCE.md` for the full contract.

## Examples

- `examples/go/raw-grpc-cache/main.go`
- `examples/go/raw-grpc-pubsub/main.go`

## Configuration

Settings come from environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `HTTP_PORT` | `8080` | HTTP admin listen port |
| `GRPC_PORT` | `9090` | gRPC data API listen port |

## Development Commands

```bash
make test
make test-race
make lint
make proto
```

Useful focused commands:

```bash
go test ./internal/store -run TestStringSetGetAndOverwrite -count=1
go test ./internal/pubsub -run TestHubPublishDeliversToActiveSubscribers -count=1
go test ./internal/server -run TestGRPCServiceDataTypesAndTTL -count=1
go test ./cmd/server -run TestPublicProtoClientAgainstRunningServer -count=1
```

## Docker

```bash
docker compose up -d
```

See `docs/ARCHITECTURE.md` for package layout.

## License

MIT - see `LICENSE`.
