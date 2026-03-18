# RESP Migration Plan

> Replace gRPC/protobuf with raw TCP + RESP protocol and I/O multiplexing (kqueue).

**Start date**: 2026-03-19
**Target**: Zero external dependencies, `redis-cli` compatible, single-threaded event loop.

---

## Phase 1: Streaming RESP Reader

Refactor `resp.go` from batch-based decoding to streaming reads over a `bufio.Reader`.

- [ ] Create `internal/core/reader.go` with `RESPReader` struct wrapping `bufio.Reader`
- [ ] Implement `ReadCommand() (*MemKVCmd, error)` — reads a complete RESP array from the stream
- [ ] Handle each RESP type: `+` simple string, `-` error, `:` integer, `$` bulk string, `*` array
- [ ] Handle inline commands (plain `PING\r\n` without RESP framing) for `redis-cli` compat
- [ ] Handle partial reads gracefully — `bufio.Reader` handles buffering, but watch for EOF mid-command
- [ ] Handle pipelining — caller should be able to call `ReadCommand()` in a loop to drain back-to-back commands
- [ ] Write unit tests: single command, pipelined commands, inline commands, malformed input
- [ ] Decide: keep or drop the custom `@` int-array type (non-standard RESP)

---

## Phase 2: Command Dispatcher

Build the router that maps Redis commands to `store`/`pubsub` operations.

- [ ] Create `internal/core/command.go` with `CommandHandler` struct
- [ ] Implement `Execute(cmd *MemKVCmd) []byte` — switches on `cmd.Cmd`, returns RESP-encoded response
- [ ] Implement string commands: `GET`, `SET` (with `EX`/`PX` options), `DEL`, `EXISTS`, `TYPE`, `EXPIRE`, `TTL`
- [ ] Implement hash commands: `HSET`, `HGET`, `HDEL`, `HGETALL`
- [ ] Implement list commands: `LPUSH`, `RPUSH`, `LPOP`, `RPOP`, `LRANGE`
- [ ] Implement set commands: `SADD`, `SREM`, `SMEMBERS`, `SISMEMBER`
- [ ] Implement pub/sub commands: `PUBLISH` (returns integer of receivers)
- [ ] Implement utility commands: `PING`, `INFO` (stats), `COMMAND` (for `redis-cli` handshake)
- [ ] Validate argument counts — return `-ERR wrong number of arguments for 'xxx' command\r\n`
- [ ] Map store errors to RESP errors (`-WRONGTYPE ...`, `-ERR ...`)
- [ ] Write unit tests for each command family

---

## Phase 3: TCP Server (Goroutine-per-Connection)

Get a working RESP server before optimizing the I/O model.

- [ ] Create `internal/server/tcp.go` with `TCPServer` struct
- [ ] Implement `NewTCPServer(addr string, handler *core.CommandHandler) (*TCPServer, error)`
- [ ] Implement `Serve()` — accept loop, one goroutine per connection
- [ ] Implement `handleConnection(conn net.Conn)` — read loop using `RESPReader`, dispatch to `CommandHandler`, write response
- [ ] Handle connection cleanup on error/disconnect
- [ ] Implement `GracefulStop()` — stop accepting, drain existing connections
- [ ] Smoke test with `redis-cli -p 9090 PING` → `PONG`
- [ ] Test `SET`/`GET`/`DEL` round-trip with `redis-cli`
- [ ] Test hash, list, set commands with `redis-cli`
- [ ] Port existing `grpc_test.go` tests to use `go-redis` client or raw TCP

---

## Phase 4: SUBSCRIBE over TCP

Wire pub/sub push delivery into the TCP connection model.

- [ ] Define subscription state on the client struct (`subscribed bool`, `subChannels`, message channel)
- [ ] Handle `SUBSCRIBE channel [channel ...]` — enter subscription mode, call `hub.Subscribe()`
- [ ] Handle `UNSUBSCRIBE [channel ...]` — leave channels, exit subscription mode if no channels left
- [ ] Push messages as RESP arrays: `*3\r\n$7\r\nmessage\r\n$<chanLen>\r\n<channel>\r\n$<payloadLen>\r\n<payload>\r\n`
- [ ] In subscription mode, only allow `SUBSCRIBE`, `UNSUBSCRIBE`, `PING`
- [ ] Handle client disconnect — call `hub.Remove(sub.ID)` to clean up
- [ ] Test with two `redis-cli` instances: one subscribes, one publishes

---

## Phase 5: I/O Multiplexing with kqueue

Replace goroutine-per-connection with a single-threaded event loop.

- [ ] Create `internal/server/eventloop.go`
- [ ] Define `Client` struct: `fd int`, `readBuf []byte`, `writeBuf []byte`, `reader *RESPReader`, subscription state
- [ ] Create kqueue FD via `syscall.Kqueue()`
- [ ] Set listener socket to non-blocking, register with `EVFILT_READ`
- [ ] Implement main event loop: `Kevent()` → process ready FDs
- [ ] On listener readable: `syscall.Accept()`, set non-blocking, register new client FD
- [ ] On client readable: `syscall.Read()`, feed to `RESPReader`, execute commands, write response
- [ ] Handle `EAGAIN` (no data), `0` (disconnect), partial reads
- [ ] Handle write buffering: if `syscall.Write()` short-writes, register `EVFILT_WRITE`, drain on next event
- [ ] Handle subscriber message delivery: pipe-based wakeup or hybrid goroutine approach
- [ ] Handle client disconnect cleanup (close FD, remove from kqueue, clean up subscriber)
- [ ] Benchmark: connections/sec, ops/sec, memory per connection vs goroutine model

---

## Phase 6: Cleanup & Finalize

Remove gRPC, update docs, and ship.

- [ ] Delete `internal/server/grpc.go`
- [ ] Delete `internal/server/grpc_test.go`
- [ ] Delete `internal/server/grpc_pubsub_test.go`
- [ ] Delete `proto/` directory
- [ ] Delete `cmd/server/public_proto_integration_test.go`
- [ ] Remove `google.golang.org/grpc` and `google.golang.org/protobuf` from `go.mod`
- [ ] Run `go mod tidy` — verify zero external dependencies
- [ ] Update `cmd/server/main.go` to use `TCPServer` instead of `GRPCServer`
- [ ] Update `.env.example` — rename `GRPC_PORT` to `PORT` (or `RESP_PORT`)
- [ ] Update `Dockerfile` to expose the single RESP port
- [ ] Update `docker-compose.yml`
- [ ] Update `docs/ARCHITECTURE.md` with new architecture diagram
- [ ] Update `docs/API_REFERENCE.md` — document Redis commands instead of gRPC methods
- [ ] Update `docs/QUICKSTART.md` — use `redis-cli` examples instead of `grpcurl`
- [ ] Update `docs/PUB_SUB.md` — document RESP-based subscribe flow
- [ ] Update `README.md`
