.PHONY: build run test

build:
	go build -ldflags="-X main.version=dev -X main.commit=$(shell git rev-parse --short HEAD 2>/dev/null || echo none)" -o bin/system-information ./cmd/system-information

run:
	go run ./cmd/system-information

test:
	go test ./...
