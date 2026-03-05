# Build stage
FROM golang:1.23-alpine AS builder

WORKDIR /app
COPY go.mod go.sum* ./
RUN go mod download

COPY . .
RUN go build -o /bin/erion-ember ./cmd/server/

# Runtime stage
FROM alpine:3.19
RUN apk add --no-cache ca-certificates
COPY --from=builder /bin/erion-ember /bin/erion-ember

EXPOSE 8080
ENTRYPOINT ["/bin/erion-ember"]
