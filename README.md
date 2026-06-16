# Internal Transfers Service

An HTTP service for internal money transfers between accounts, backed by
PostgreSQL. Built with Go and the [Hertz](https://github.com/cloudwego/hertz)
HTTP framework.

## Features

- Create accounts, query balances, and transfer money between accounts.
- **High-precision amounts** (no floats): `NUMERIC(38,18)` in Postgres and
  `shopspring/decimal` in Go. Amounts are exchanged as JSON strings.
- **Strong consistency**: each transfer runs in one transaction with
  `SELECT ... FOR UPDATE` row locks (acquired in ascending id order to avoid
  deadlocks) plus a `CHECK (balance >= 0)` safety net.
- **Double-entry ledger** for full auditability: every transfer writes a debit
  and a credit entry, each with a `balance_after` snapshot, so any account's
  history can be replayed.
- **Idempotency** via the optional `Idempotency-Key` HTTP header — retries never
  move money twice.
- **Observability**: Prometheus metrics at `/metrics`, plus liveness and
  readiness probes.
- **Graceful shutdown**: on SIGTERM the service drains in-flight requests and
  reports `not ready` so a load balancer stops sending new traffic.

## API

The service listens on two ports: a **public** port for business endpoints and
an **internal admin** port for metrics and health probes. The admin port should
not be exposed to the public network — its metrics reveal internal state.

Public port (`APP_ADDR`, default `:8080`):

| Method | Path                       | Description                       | Success |
|--------|----------------------------|-----------------------------------|---------|
| POST   | `/accounts`                | Create an account                 | 201     |
| GET    | `/accounts/{account_id}`   | Get an account and its balance    | 200     |
| POST   | `/transactions`            | Transfer between two accounts     | 201     |

Internal admin port (`METRICS_ADDR`, default `:9090`):

| Method | Path                       | Description                       | Success |
|--------|----------------------------|-----------------------------------|---------|
| GET    | `/livez`                   | Liveness probe                    | 200     |
| GET    | `/readyz`                  | Readiness probe (checks Postgres) | 200/503 |
| GET    | `/metrics`                 | Prometheus metrics                | 200     |

### Create account

```bash
curl -X POST localhost:8080/accounts \
  -H 'Content-Type: application/json' \
  -d '{"account_id": 123, "initial_balance": "100.23344"}'
```

### Get account

```bash
curl localhost:8080/accounts/123
# {"account_id":123,"balance":"100.23344"}
```

### Transfer

```bash
curl -X POST localhost:8080/transactions \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: 550e8400-e29b-41d4-a716-446655440000' \
  -d '{"source_account_id": 123, "destination_account_id": 456, "amount": "100.12345"}'
```

The `Idempotency-Key` header is optional. When supplied, retrying the same
request returns the original result without moving money again; reusing a key
with different parameters returns `422`.

**Design note — why optional, not required.** A transfer is a money-moving
operation that clients retry on timeout, so idempotency is the mechanism that
prevents a retry from double-charging. That protection only works if the client
sends the *same* key on every retry. A stricter design would make the header
**required** and reject requests without it (`400`), forcing every caller to
take responsibility for a stable key — this is what production payment systems
(Stripe, PayPal) effectively do. We keep it **optional** here to stay easy to
call (curl/Postman work with no setup); the idempotency *mechanism* itself is
fully implemented regardless (a hit returns the original transfer; a parameter
mismatch returns `422`). For a real production deployment, the recommendation is
to make the header required and enforce it at the API edge.

### API documentation (kept in sync)

The full contract lives in [`api/openapi.yaml`](api/openapi.yaml) (OpenAPI 3).
It is the source of truth, and an integration test
(`test/integration/openapi_test.go`) drives the real router for success and
every error case and validates each response against the spec — so if a handler
returns a status or shape the doc does not declare, the test fails. Editing the
API therefore forces editing the doc.

Browse it via Swagger UI (started by `make docker-up`) at
http://localhost:8081, or render locally with any OpenAPI viewer.

### Error responses

Errors return a JSON body `{"error": "..."}` with an appropriate status:

| Status | Cases |
|--------|-------|
| 400 | invalid JSON, non-positive or too-precise amount, source equals destination, malformed idempotency key |
| 404 | account not found (the message names the missing id, e.g. `account 123 not found`) |
| 409 | account already exists, insufficient funds |
| 422 | idempotency key reused with different parameters |
| 500 | unexpected server/database error |

## Requirements

- Go 1.26+
- PostgreSQL 16+ (or Docker)

## Quick start (Docker)

```bash
make docker-up        # builds the image, starts Postgres, the service,
                      # Prometheus and Grafana
```

Business endpoints are on `:8080`; metrics and probes on `:9090`. Migrations
run automatically on startup. The compose stack also brings up an observability
stack:

| Service | URL | Notes |
|---------|-----|-------|
| Service (business) | http://localhost:8080 | the API |
| Service (admin) | http://localhost:9090/metrics | metrics + probes |
| Prometheus | http://localhost:9090 (container) / mapped host port | scrapes `app:9090` |
| Grafana | http://localhost:3000 | anonymous access; Prometheus datasource and an "Internal Transfers" dashboard are auto-provisioned |

If some of these host ports are already taken on your machine, add a
`deployments/docker-compose.local.yml` with `ports: !override` entries and run
`docker compose -f deployments/docker-compose.yml -f deployments/docker-compose.local.yml up -d`.

## Run locally (without Docker)

Run the service directly on your machine against a local Postgres. This needs
only Go and a reachable Postgres; migrations run automatically on startup.

**1. Start Postgres.** Use an existing instance, or spin up a throwaway one:

```bash
docker run -d --name transfers-pg \
  -e POSTGRES_USER=postgres -e POSTGRES_PASSWORD=postgres -e POSTGRES_DB=transfers \
  -p 5432:5432 postgres:16-alpine
```

**2. Configure.** Copy the sample env and point `DATABASE_URL` at your Postgres:

```bash
cp .env.example .env          # edit DATABASE_URL if your Postgres differs
export $(grep -v '^#' .env | xargs)
```

**3. Build and run.** Either run from source or build a binary:

```bash
make run                      # go run ./cmd/server
# or:
make build && ./bin/server    # compiled binary
```

On startup the service applies migrations (`RUN_MIGRATIONS=true`), then listens
on `APP_ADDR` (`:8080`) for business endpoints and `METRICS_ADDR` (`:9090`) for
metrics and probes.

**4. Smoke test.**

```bash
curl -X POST localhost:8080/accounts \
  -H 'Content-Type: application/json' \
  -d '{"account_id": 1, "initial_balance": "100"}'

curl -X POST localhost:8080/accounts \
  -H 'Content-Type: application/json' \
  -d '{"account_id": 2, "initial_balance": "0"}'

curl -X POST localhost:8080/transactions \
  -H 'Content-Type: application/json' \
  -d '{"source_account_id": 1, "destination_account_id": 2, "amount": "30"}'

curl localhost:8080/accounts/1   # {"account_id":1,"balance":"70"}
curl localhost:8080/accounts/2   # {"account_id":2,"balance":"30"}

curl localhost:9090/readyz       # {"status":"ready","checks":{"postgres":"ok"}}
```

To tear down the throwaway Postgres: `docker rm -f transfers-pg`.

## Testing

```bash
make test             # unit tests (no database needed)

# Integration tests need a Postgres instance. Point this at a dedicated,
# throwaway database — the tests TRUNCATE tables on each run:
export TEST_DATABASE_URL=postgres://postgres:postgres@localhost:5432/transfers_test?sslmode=disable
make test-integration # transfer, concurrency, and idempotency tests
```

Unit tests cover the transfer orchestration (validation, insufficient funds,
idempotent replay, conflicts) using in-memory fakes. Integration tests run
against a real Postgres and include a concurrency test that fires more transfers
than the balance can cover, asserting no overdraft and conservation of funds.

## Configuration

All configuration is via environment variables (see `.env.example`):

| Variable | Default | Description |
|----------|---------|-------------|
| `APP_ADDR` | `:8080` | Public HTTP listen address (business endpoints) |
| `METRICS_ADDR` | `:9090` | Internal admin address (metrics + probes) |
| `DATABASE_URL` | — (required) | PostgreSQL DSN |
| `DB_MAX_CONNS` | `10` | Connection pool size |
| `RUN_MIGRATIONS` | `true` | Run migrations on startup |
| `SHUTDOWN_TIMEOUT_SECONDS` | `15` | Graceful shutdown budget |
| `LOG_LEVEL` | `info` | `debug` / `info` / `warn` / `error` |
| `LOG_FILE` | — | If set, JSON logs are also written to this file (mounted to `deployments/logs/` under Docker) |

## Project layout

```
cmd/server              entry point and graceful shutdown
internal/domain         entities and domain errors
internal/repository     data access (pgx), transaction helper, error mapping
internal/service        business logic (accounts, transfers, money validation)
internal/api            HTTP handlers, routing, middleware, health probes
internal/observability  Prometheus metrics and logging
internal/platform       Postgres pool and migration runner
migrations              SQL schema
test/integration        end-to-end tests (transfer, concurrency, idempotency)
```

## Data model

- `accounts` — current balance snapshot, with a non-negative `CHECK`.
- `transfers` — immutable log of each transfer; holds the optional idempotency
  key under a partial unique index.
- `ledger_entries` — two rows per transfer (debit + credit) with a
  `balance_after` snapshot for audit and replay.

Auditing a single account's history:

```sql
SELECT created_at, direction, amount, balance_after
FROM ledger_entries
WHERE account_id = $1
ORDER BY id;
```

Sanity check that the ledger balances globally:

```sql
SELECT
  SUM(amount) FILTER (WHERE direction = 'debit')  AS total_debit,
  SUM(amount) FILTER (WHERE direction = 'credit') AS total_credit
FROM ledger_entries;  -- the two totals must be equal
```

## Assumptions

- A single shared currency for all accounts; amounts use up to 18 decimal places.
- No authentication or authorization (out of scope).
- `account_id` is supplied by the client and is the primary key; re-creating an
  existing id returns 409.
- The `Idempotency-Key` header is optional; without it a transfer is processed
  non-idempotently. The request body matches the spec exactly — idempotency is
  carried entirely in the header. See the design note under
  [Transfer](#transfer) for why it is optional rather than required, and what a
  production deployment should do instead.
- Because the system is a single Postgres instance with synchronous
  transactions, a transfer is either fully committed or fully rolled back; there
  is no need for a pending state, two-phase commit, or sagas.

## Production & AWS considerations

Not implemented here (out of scope for this service), but worth noting:

- **Migrations under multiple replicas**: the migration runner takes a Postgres
  advisory lock so concurrent instances starting together are safe. In
  production, running migrations as a separate one-off job is recommended.
- **Connection limits**: with N replicas, `DB_MAX_CONNS × N` must stay under the
  database `max_connections`. RDS Proxy is a good fit for pooling and failover.
- **Secrets**: inject `DATABASE_URL` from Secrets Manager / SSM, not a committed
  file. RDS IAM authentication avoids long-lived passwords.
- **TLS**: use `sslmode=require` (or `verify-full` with the RDS CA) in production.
- **Port separation**: business endpoints and the admin endpoints (`/metrics`,
  `/livez`, `/readyz`) listen on different ports. Expose only the public port
  through the ALB; keep the admin port (`METRICS_ADDR`) reachable only from
  inside the VPC/cluster (security group / NetworkPolicy), since metrics expose
  internal state (Go version, traffic, error and business volumes).
- **Health checks**: the kubelet / ALB health check targets `/readyz` on the
  admin port; use `/livez` for the orchestrator's restart decisions.
- **Metrics**: Prometheus scrapes `/metrics` on the admin port from within the
  cluster, or export to CloudWatch / AMP via the OpenTelemetry Collector.
