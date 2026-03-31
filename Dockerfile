FROM golang:1.23-alpine AS builder

ENV CGO_ENABLED=0

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -ldflags="-s -w" -o /bin/gomemkv ./cmd/server/

FROM alpine:3.19
RUN apk add --no-cache ca-certificates && \
    addgroup -S gomemkv && \
    adduser -S -G gomemkv -h /app gomemkv

ENV PORT=9090

WORKDIR /app

COPY --from=builder /bin/gomemkv /bin/gomemkv

EXPOSE 9090
USER gomemkv:gomemkv
ENTRYPOINT ["/bin/gomemkv"]
