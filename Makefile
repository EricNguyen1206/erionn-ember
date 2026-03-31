.PHONY: build test clean run lint test-verbose test-race docker-build release-validate

BIN := bin/gomemkv

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


## docker-build: build the container image locally
docker-build:
	docker build -t gomemkv:local .

## release-validate: run release validation checklist
release-validate: build test test-race lint

.DEFAULT_GOAL := build
