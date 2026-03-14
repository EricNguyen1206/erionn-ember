# Build stage - pure Go, CGO_ENABLED=0, smallest possible binary
FROM golang:1.23-alpine AS builder

ENV CGO_ENABLED=0

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -ldflags="-s -w" -o /bin/erion-ember ./cmd/server/

# Runtime stage - minimal Alpine image with non-root user
FROM alpine:3.19
RUN apk add --no-cache ca-certificates && \
    addgroup -S ember && \
    adduser -S -G ember -h /app ember

ENV HTTP_PORT=8080 \
    GRPC_PORT=9090 \
    CACHE_SIMILARITY_THRESHOLD=0.85 \
    CACHE_MAX_ELEMENTS=100000 \
    CACHE_DEFAULT_TTL=3600

WORKDIR /app

COPY --from=builder /bin/erion-ember /bin/erion-ember

EXPOSE 8080 9090
USER ember:ember
ENTRYPOINT ["/bin/erion-ember"]
