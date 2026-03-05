# Build stage — no CGO needed (pure Go)
FROM golang:1.23-alpine AS builder

WORKDIR /app
COPY . .
RUN go mod tidy && \
    go build -ldflags="-s -w" -o /bin/erion-ember ./cmd/server/

# Runtime stage — minimal Alpine image
FROM alpine:3.19
RUN apk add --no-cache ca-certificates
COPY --from=builder /bin/erion-ember /bin/erion-ember

EXPOSE 8080
ENTRYPOINT ["/bin/erion-ember"]
