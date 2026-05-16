.PHONY: build run test lint tidy up down logs docker/up docker/down docker/logs docker/restart-simulation docker/restart-fitness

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

docker/up:
	docker compose up --build -d

docker/down:
	docker compose down

docker/logs:
	docker compose logs -f controller

docker/restart-simulation:
	docker compose exec redis redis-cli FLUSHALL
	docker compose up --build --force-recreate -d --no-deps controller fitness-calc client-0 client-1 client-2 client-3 client-4

docker/restart-fitness:
	docker compose up --build --force-recreate -d --no-deps fitness-calc
