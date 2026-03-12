# Build stage — pure Go, CGO_ENABLED=0, smallest possible binary
FROM golang:1.23-alpine AS builder

ENV CGO_ENABLED=0

WORKDIR /app
COPY . .
RUN go mod tidy && \
    go build -ldflags="-s -w" -o /bin/erion-ember ./cmd/server/

# Runtime stage — minimal scratch-like image
FROM alpine:3.19
RUN apk add --no-cache ca-certificates

COPY --from=builder /bin/erion-ember /bin/erion-ember

EXPOSE 8080 9090
ENTRYPOINT ["/bin/erion-ember"]
