# Architecture

Single-process Go service with one shared in-memory keyspace and one shared pub/sub hub.

## Pieces

- RESP/TCP protocol (`redis-cli` compatible)
- one `store.Store` for strings, hashes, lists, sets, and TTL
- one `pubsub.Hub` for channel subscriptions and fan-out

## Components

### `cmd/server`

- loads `PORT` env var (default `9090`)
- creates the shared store and pub/sub hub
- starts RESP/TCP server
- handles graceful shutdown on SIGTERM/SIGINT, prints stats

### `internal/server`

**`server.go`** — `TCPServer`: accept loop, goroutine-per-connection

**`client.go`** — per-connection state:
- normal mode: dispatches commands via `CommandHandler`
- subscription mode: only SUBSCRIBE/UNSUBSCRIBE/PING allowed, pushes messages asynchronously

### `internal/core`

**`reader.go`** — `RESPReader`: streaming command parser (RESP arrays + inline commands)

**`cmd_handler/handler.go`** — `CommandHandler`: maps RESP commands to store/pubsub operations

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

- string → `string`
- hash → `map[string]string`
- list → `[]string`
- set → `map[string]struct{}`

## Request Flow

### String read

1. Client sends `GET key` over TCP (RESP)
2. `RESPReader` parses the command
3. `CommandHandler` calls `store.GetString(key)`
4. Store checks expiration, type, and value
5. Handler writes RESP-encoded response back to client

### Pub/sub

1. Client sends `SUBSCRIBE channel` over TCP
2. `client.go` registers a subscriber in the hub, enters subscription mode
3. Another client sends `PUBLISH channel payload`
4. Hub fans the message to all active subscribers
5. `pushMessages()` goroutine writes RESP message push to the subscriber's TCP connection

## Concurrency

- `store.Store` uses a mutex around keyspace mutation and lazy expiration
- `pubsub.Hub` uses its own mutex for subscription bookkeeping
- slow subscribers are removed instead of buffering unbounded data
- `client.writeMu` protects concurrent writes from command loop and message push goroutine

## Operational Shape

- single node
- memory only
- no persistence
- no auth
- RESP/TCP, `redis-cli` compatible

That is the whole runtime model.
