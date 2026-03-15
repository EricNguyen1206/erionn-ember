# erionn-ember

grpc cache server in go.

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
- resp

ports:
- http `8080`
- grpc `9090`

run:

```bash
make build
./bin/erion-ember
```

check:

```bash
curl http://localhost:8080/health
curl http://localhost:8080/ready
```

examples:

```bash
go run ./examples/go/raw-grpc-cache
go run ./examples/go/raw-grpc-pubsub
```

dev:

```bash
make test
make test-race
make lint
make proto
```

more:
- `proto/ember/v1/cache.proto`
- `docs/API_REFERENCE.md`
- `docs/ARCHITECTURE.md`
