.PHONY: build test clean run

BIN := bin/erion-ember

## build: compile the server binary
build:
	go build -o $(BIN) ./cmd/server/

## test: run all tests
test:
	go test ./...

## test-verbose: run tests with output
test-verbose:
	go test -v ./...

## test-race: run tests with race detector
test-race:
	go test -race ./...

## run: build and run the server
run: build
	$(BIN)

## clean: remove build artifacts
clean:
	rm -rf bin/

## generate: regenerate protobuf code (requires protoc + plugins)
## Install: brew install protoc-gen-go protoc-gen-go-grpc
##          go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway@latest
generate:
	mkdir -p gen/ember/v1
	protoc -I proto \
		-I third_party/googleapis \
		--go_out=gen --go_opt=paths=source_relative \
		--go-grpc_out=gen --go-grpc_opt=paths=source_relative \
		--grpc-gateway_out=gen --grpc-gateway_opt=paths=source_relative \
		proto/ember/v1/cache.proto

## lint: run golangci-lint
lint:
	golangci-lint run ./...

.DEFAULT_GOAL := build
