# ── Builder ─────────────────────────────────────────────────────────────────
# CGO_ENABLED=1 required for hugot (wraps ONNX Runtime C library).
# Using Debian (glibc) because ONNX Runtime is not compatible with Alpine (musl).
FROM golang:1.23 AS builder

ENV CGO_ENABLED=1

WORKDIR /app
COPY . .
RUN go mod tidy && \
    go build -ldflags="-s -w" -o /bin/erion-ember ./cmd/server/

# ── Runtime ──────────────────────────────────────────────────────────────────
FROM debian:bookworm-slim

RUN apt-get update && \
    apt-get install -y --no-install-recommends ca-certificates && \
    rm -rf /var/lib/apt/lists/*

COPY --from=builder /bin/erion-ember /bin/erion-ember

EXPOSE 8080

# MODEL_DIR is auto-downloaded by the server on first start if empty directory.
# Mount a persistent volume for the models to avoid re-downloading:
#   docker run -v $(pwd)/models:/models -e MODEL_DIR=/models ...
ENTRYPOINT ["/bin/erion-ember"]
