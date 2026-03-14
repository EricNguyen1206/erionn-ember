.PHONY: build test clean run lint test-verbose test-race proto proto-tools

BIN := bin/erion-ember
MODULE := github.com/EricNguyen1206/erion-ember
PROTO_FILES := proto/ember/v1/semantic_cache.proto
PROTOC_GEN_GO := $(abspath bin/protoc-gen-go)
PROTOC_GEN_GO_GRPC := $(abspath bin/protoc-gen-go-grpc)

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

## lint: run golangci-lint
lint:
	golangci-lint run ./...

## proto-tools: install protobuf code generators locally
proto-tools:
	GOBIN=$(abspath bin) go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.6
	GOBIN=$(abspath bin) go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5.1

## proto: generate protobuf and gRPC bindings
proto: proto-tools
	protoc --proto_path=. --plugin=protoc-gen-go=$(PROTOC_GEN_GO) --plugin=protoc-gen-go-grpc=$(PROTOC_GEN_GO_GRPC) --go_out=. --go_opt=module=$(MODULE) --go-grpc_out=. --go-grpc_opt=module=$(MODULE) $(PROTO_FILES)

.DEFAULT_GOAL := build
