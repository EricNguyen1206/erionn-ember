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
- grpc `9090`

run:

```bash
make build
./bin/erionn-ember
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
