FROM golang:1.23-alpine AS builder

ENV CGO_ENABLED=0

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -ldflags="-s -w" -o /bin/erionn-ember ./cmd/server/

FROM alpine:3.19
RUN apk add --no-cache ca-certificates && \
    addgroup -S ember && \
    adduser -S -G ember -h /app ember

ENV HTTP_PORT=8080 \
	GRPC_PORT=9090

WORKDIR /app

COPY --from=builder /bin/erionn-ember /bin/erionn-ember

EXPOSE 8080 9090
USER ember:ember
ENTRYPOINT ["/bin/erionn-ember"]
