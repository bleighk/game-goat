.PHONY: build test lint install clean

BIN_EXT := $(if $(filter windows,$(shell go env GOOS)),.exe,)

build:
	go build -trimpath -o bin/game-goat-pp-cli$(BIN_EXT) ./cmd/game-goat-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/game-goat-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -trimpath -o bin/game-goat-pp-mcp$(BIN_EXT) ./cmd/game-goat-pp-mcp

install-mcp:
	go install ./cmd/game-goat-pp-mcp

build-all: build build-mcp
