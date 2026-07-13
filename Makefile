.PHONY: build run test lint

build:
	go build -o bin/resolver ./cmd/resolver

run: build
	./bin/resolver

test:
	go test -race ./...

lint:
	golangci-lint run
