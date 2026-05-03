.PHONY: build run test lint tidy up down logs

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

up:
	docker compose up --build

down:
	docker compose down

logs:
	docker compose logs -f controller
