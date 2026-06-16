.PHONY: run build test test-integration lint tidy docker-up docker-down

# Base compose file, plus an optional machine-local override that remaps host
# ports to avoid clashing with other stacks. The override is gitignored, so it
# is only layered in when present (this machine); CI and clean checkouts use the
# base file alone.
# 基础 compose 文件，外加一个可选的本机端口覆盖（避免与其他栈端口冲突）。
# 该覆盖文件被 gitignore，仅在存在时（本机）叠加；CI 与全新检出只用基础文件。
COMPOSE_FILES := -f deployments/docker-compose.yml
ifneq (,$(wildcard deployments/docker-compose.local.yml))
COMPOSE_FILES += -f deployments/docker-compose.local.yml
endif

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
	docker compose $(COMPOSE_FILES) up --build

docker-down:
	docker compose $(COMPOSE_FILES) down -v
