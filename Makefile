.PHONY: run build test test-integration lint tidy docker-up docker-down migrate

run:
	go run ./cmd/server

build:
	go build -o bin/server ./cmd/server

test:
	go test ./...

test-integration:
	@test -n "$(TEST_DATABASE_URL)" || (echo "set TEST_DATABASE_URL to a Postgres DSN" && exit 1)
	go test -tags=integration -race ./test/...

lint:
	go vet ./...

tidy:
	go mod tidy

docker-up:
	docker compose -f deployments/docker-compose.yml up --build

docker-down:
	docker compose -f deployments/docker-compose.yml down -v
