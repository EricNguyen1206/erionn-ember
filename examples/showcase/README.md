## Showcase Demo

This sandbox gives you a near-one-command release demo for Erion Ember using Docker Compose.

Prerequisites:

- Docker with `docker compose`
- `curl`
- optional `jq` for prettier JSON output

From the repository root:

```bash
./examples/showcase/scripts/run-demo.sh
```

What it does:

- Starts a dedicated demo stack on HTTP `18080` and gRPC `19090`
- Builds the local release image as `erion-ember:local`
- Waits for the service to report ready
- Shows a cache miss for a realistic chat prompt
- Stores a placeholder LLM answer
- Repeats the lookup to show a cache hit
- Prints `/v1/stats` and selected `/metrics` lines before and after the demo

Stop the showcase stack when you are done:

```bash
docker compose -f examples/showcase/docker-compose.yml down
```

Notes:

- The script keeps the container running after the demo so you can keep exploring
- If you already have something on `18080` or `19090`, stop it or adjust the compose ports first
