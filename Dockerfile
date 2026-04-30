FROM golang:1.23-alpine AS builder

ENV CGO_ENABLED=0

ARG VERSION=dev
ARG COMMIT=none
ARG DATE=unknown

WORKDIR /app
COPY go.mod go.sum* ./
RUN go mod download 2>/dev/null || true

COPY . .
RUN go build -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${DATE}" -o /bin/gomemkv ./cmd/gomemkv/

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
