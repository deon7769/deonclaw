.PHONY: build test fmt

build:
	go build -o bin/deonctl ./cmd/deonctl

test:
	go test ./...

fmt:
	gofmt -w .
