# Architecture

Single-process Go service with one shared in-memory keyspace and one shared pub/sub hub.

## Pieces

- gRPC for data commands
- HTTP for health and metrics
- one `store.Store` for strings, hashes, lists, sets, and TTL
- one `pubsub.Hub` for channel subscriptions and fan-out

## Components

### `cmd/server`

- loads `HTTP_PORT` and `GRPC_PORT`
- creates the shared store and pub/sub hub
- starts HTTP and gRPC servers
- handles graceful shutdown

### `internal/server/grpc.go`

- validates requests
- translates store errors into gRPC status codes
- maps one RPC to one clear store or pub/sub operation
- keeps transport logic thin

### `internal/server/http.go`

- serves `/health`, `/ready`, and `/metrics`
- keeps request metrics for admin visibility
- does not expose data commands

### `internal/store`

- owns the keyspace `map[string]*Entry`
- stores key type, value, timestamps, and optional expiration
- implements generic key operations plus datatype-specific commands
- lazily removes expired keys when touched

### `internal/pubsub`

- tracks subscribers by channel
- publishes only to currently connected subscribers
- drops slow subscribers when their buffer is full

## Data Model

Each key is stored as an `Entry` with:

- `Key`
- `Type`
- `Value`
- `ExpiresAt`
- `CreatedAt`
- `UpdatedAt`

Supported value shapes:

- string -> `string`
- hash -> `map[string]string`
- list -> `[]string`
- set -> `map[string]struct{}`

## Request Flow

### String read

1. Client sends `Get(key)` over gRPC
2. gRPC handler validates the key
3. Handler calls `store.GetString(key)`
4. Store checks expiration, type, and value
5. Handler returns `found/value`

### Pub/sub

1. Client opens `Subscribe(channels)` stream
2. Server registers a subscriber in the hub
3. Another client calls `Publish(channel, payload)`
4. Hub fans the message out to active subscribers only
5. gRPC stream sends `SubscribeMessage`

## Concurrency

- `store.Store` uses a mutex around keyspace mutation and lazy expiration
- `pubsub.Hub` uses its own mutex for subscription bookkeeping
- slow subscribers are removed instead of buffering unbounded data

## Operational Shape

- single node
- memory only
- no persistence
- no auth
- no RESP compatibility layer

That is the whole runtime model.
