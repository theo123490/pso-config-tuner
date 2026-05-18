.PHONY: build run test lint tidy

BINARY := bin/controller

build:
	go build -o $(BINARY) ./cmd/controller

run: build
	./$(BINARY)

test:
	go test ./...

lint:
	go vet ./...

tidy:
	go mod tidy
