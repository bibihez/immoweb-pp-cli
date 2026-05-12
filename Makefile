.PHONY: build test lint install clean

build:
	go build -o bin/immoweb-pp-cli ./cmd/immoweb-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/immoweb-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/immoweb-pp-mcp ./cmd/immoweb-pp-mcp

install-mcp:
	go install ./cmd/immoweb-pp-mcp

build-all: build build-mcp
