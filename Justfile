set positional-arguments

# FitCore development task runner.

export DATABASE_URL := env_var_or_default("DATABASE_URL", "postgres://fitcore:fitcore@localhost:5432/fitcore?sslmode=disable")
export TEST_DATABASE_URL := env_var_or_default("TEST_DATABASE_URL", "postgres://fitcore:fitcore@localhost:5432/fitcore_test?sslmode=disable")
export TEST_REDIS_URL := env_var_or_default("TEST_REDIS_URL", "redis://localhost:6379/1")
export LOAD_BASE := env_var_or_default("LOAD_BASE", "http://localhost:8081")
export LOAD_DATABASE_URL := env_var_or_default("LOAD_DATABASE_URL", "postgres://fitcore:fitcore@localhost:5433/fitcore?sslmode=disable")
export LOAD_ADMIN := env_var_or_default("LOAD_ADMIN", "loadadmin@fitcore.load")
export LOAD_PASSWORD := env_var_or_default("LOAD_PASSWORD", "Password1!")

# Show available recipes
default:
    @just --list

# Compile the server and CLI binaries
build:
    go build ./...

# Run the API server locally
run:
    go run ./cmd/server

# Format all Go sources
fmt:
    gofmt -w $(find . -name '*.go')

# Verify formatting without modifying files
fmt-check:
    @test -z "$(gofmt -l cmd internal api)" || (echo "gofmt needed:"; gofmt -l cmd internal api; exit 1)

# Static analysis
vet:
    go vet ./...

# Lint (requires golangci-lint)
lint:
    golangci-lint run ./...

# Regenerate OpenAPI server code from api/openapi.yaml
gen-api:
    go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.8.0 -config oapi-codegen.yaml api/openapi.yaml

# Fail if generated code is out of date with api/openapi.yaml
gen-check:
    @tmp="$(mktemp -d)" && \
    trap 'rm -rf "$tmp"' EXIT && \
    sed "s#^output:.*#output: $tmp/openapi.gen.go#" oapi-codegen.yaml > "$tmp/cfg.yaml" && \
    go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.8.0 -config "$tmp/cfg.yaml" api/openapi.yaml && \
    diff -q internal/httpapi/openapi/openapi.gen.go "$tmp/openapi.gen.go" && \
    echo "openapi code is up to date"

# Run the full verification pipeline (local equivalent of CI)
check:
    just fmt-check
    just vet
    just lint
    just build
    just test
    just gen-check

# Scan Go dependencies for known vulnerabilities (also runs in CI)
vuln:
    go run golang.org/x/vuln/cmd/govulncheck@latest ./...

# Install git pre-commit hooks (core.hooksPath = .githooks)
install-hooks:
    git config core.hooksPath .githooks
    @echo "pre-commit hooks installed"

# Run unit tests
test:
    go test ./...

# Run integration tests (requires TEST_DATABASE_URL to point at a live database)
test-integration:
    go test -tags integration ./...

# Apply pending migrations
migrate-up:
    go run ./cmd/migrate -command up

# Roll back the last N migrations (default 1)
migrate-down steps='1':
    go run ./cmd/migrate -command down -steps {{ steps }}

# Show current schema version
migrate-version:
    go run ./cmd/migrate -command version

# Create a new migration pair: just migrate-create name=add_things
migrate-create name:
    go run ./cmd/migrate -command create -name {{ name }}

# Start the full dev stack
compose-up:
    docker compose -f deploy/docker-compose.yml --env-file .env up -d

# Stop the dev stack
compose-down:
    docker compose -f deploy/docker-compose.yml --env-file .env down

# Seed the staff accounts used by the smoke flows (and reports).
# set-password reads the password from stdin (never as a flag); printf avoids
# leaking it with a trailing newline or shell history entry.
seed-smoke email='admin@fitcore.local' password='Password1!':
    @printf '%s' '{{ password }}' | go run ./cmd/set-password -email {{ email }} -perms 'branches:read,branches:create,branches:update,members:read,members:create,members:update,members:delete,memberships:read,memberships:create,memberships:update,packages:read,packages:create,packages:update,classes:read,classes:create,classes:update,classes:delete,bookings:read,bookings:create,bookings:update,attendance:read,attendance:create,attendance:update,billing:read,billing:create,billing:update,staff:read,staff:create,staff:update,trainers:read,trainers:create,trainers:update'
    @printf '%s' '{{ password }}' | go run ./cmd/set-password -email viewer@fitcore.local -perms 'branches:read'

# Run manual smoke flows against the live stack (scripts/smoke/): requires
# compose-up + migrations applied; seeds the auth-flow staff account on demand.
smoke email='admin@fitcore.local' password='Password1!':
    just seed-smoke {{ email }} {{ password }}
    @scripts/smoke/health_flow.sh
    @SMOKE_EMAIL={{ email }} SMOKE_PASSWORD={{ password }} scripts/smoke/auth_flow.sh
    @scripts/smoke/branches_flow.sh
    @scripts/smoke/members_flow.sh
    @scripts/smoke/packages_flow.sh
    @scripts/smoke/memberships_flow.sh
    @scripts/smoke/classes_flow.sh
    @scripts/smoke/bookings_flow.sh
    @scripts/smoke/attendance_flow.sh
    @scripts/smoke/billing_flow.sh
    @scripts/smoke/staff_flow.sh
    @scripts/smoke/trainers_flow.sh

# Produce a full test report (scripts/report-tests.sh): unit+integration tests
# with coverage, race detection, pass/fail counts + reasons, then the smoke
# flows. Exit nonzero on any failure. Requires compose-up + migrations applied
# and the API running (just run). Options: --export [DIR] writes the report to
# DIR/test-report-<timestamp>.txt (DIR defaults to artifacts).
report *args:
    @scripts/report-tests.sh {{ args }}

# Start the isolated load-testing stack (deploy/docker-compose.load.yml).
# NOTE: this project is named fitcore-load and uses its own postgres/redis
# volumes; it does not touch the dev stack. It publishes postgres on
# 127.0.0.1:5433 and the API on 127.0.0.1:8081.
load-up:
    docker compose -f deploy/docker-compose.load.yml up -d --build

# Stop the load-testing stack (containers removed, volumes kept)
load-down:
    docker compose -f deploy/docker-compose.load.yml down

# Remove the load-testing stack and all of its volumes (full reset)
load-reset:
    docker compose -f deploy/docker-compose.load.yml down -v

# Apply migrations to the load-test database
load-migrate:
    DATABASE_URL='{{ LOAD_DATABASE_URL }}' go run ./cmd/migrate -command up

# Seed the load-test admin account (password read from stdin, never as a flag)
load-seed-admin email=LOAD_ADMIN password=LOAD_PASSWORD:
    @printf '%s' '{{ password }}' | DATABASE_URL='{{ LOAD_DATABASE_URL }}' go run ./cmd/set-password -email {{ email }} -perms 'branches:read,branches:create,branches:update,members:read,members:create,members:update,members:delete,memberships:read,memberships:create,memberships:update,packages:read,packages:create,packages:update,classes:read,classes:create,classes:update,classes:delete,bookings:read,bookings:create,bookings:update,attendance:read,attendance:create,attendance:update,billing:read,billing:create,billing:update,staff:read,staff:create,staff:update,trainers:read,trainers:create,trainers:update'

# Seed the load fixture: 5 branches x 300 members, packages, classes and
# memberships through the API. Requires load-up + load-migrate + load-seed-admin.
load-seed capacity='30':
    @cd scripts/load && go run . -mode seed -base '{{ LOAD_BASE }}' -admin '{{ LOAD_ADMIN }}' -password '{{ LOAD_PASSWORD }}' -capacity {{ capacity }} -out ../../artifacts/load

# Warm the pooled connections and caches before measured scenarios
load-warmup:
    @cd scripts/load && go run . -mode warmup -base '{{ LOAD_BASE }}' -admin '{{ LOAD_ADMIN }}' -password '{{ LOAD_PASSWORD }}' -out ../../artifacts/load

# Validate the SLO (default: p95 < 400ms, err < 0.5%, mixed realistic mix)
load-slo rate='150' duration='5m':
    @cd scripts/load && go run . -mode slo -base '{{ LOAD_BASE }}' -admin '{{ LOAD_ADMIN }}' -password '{{ LOAD_PASSWORD }}' -rate {{ rate }} -duration {{ duration }} -out ../../artifacts/load

# Ramp from 5 to 400 RPS in steps to find the saturation point
load-break:
    @cd scripts/load && go run . -mode ramp -base '{{ LOAD_BASE }}' -admin '{{ LOAD_ADMIN }}' -password '{{ LOAD_PASSWORD }}' -out ../../artifacts/load

# Prove the concurrency invariants (booking capacity exactly-N, check-in race,
# invoice pay race) under contention
load-correctness capacity='30':
    @cd scripts/load && go run . -mode correctness -base '{{ LOAD_BASE }}' -admin '{{ LOAD_ADMIN }}' -password '{{ LOAD_PASSWORD }}' -capacity {{ capacity }} -out ../../artifacts/load

# Long soak: SLO mix in 4 consecutive windows watching latency drift + memory
load-soak rate='100' duration='45m':
    @cd scripts/load && go run . -mode soak -base '{{ LOAD_BASE }}' -admin '{{ LOAD_ADMIN }}' -password '{{ LOAD_PASSWORD }}' -rate {{ rate }} -duration {{ duration }} -out ../../artifacts/load

# ---- production (deploy/docker-compose.prod.yml + .env) -----------------

# Validate and start the production stack. Requires .env (see deploy/.env.prod.example).
prod-up:
    set -a; . ./.env; set +a
    docker compose -f deploy/docker-compose.prod.yml up -d --build

# Stop the production stack (volumes kept)
prod-down:
    docker compose -f deploy/docker-compose.prod.yml down

# Stream production logs (all services, follow)
prod-logs *svc:
    docker compose -f deploy/docker-compose.prod.yml logs -f --tail 100 {{ svc }}

# Show production stack status
prod-status:
    docker compose -f deploy/docker-compose.prod.yml ps

# Apply pending migrations to the production database (one-shot migrate service)
prod-migrate:
    docker compose -f deploy/docker-compose.prod.yml run --rm migrate

# Nightly full backup (pg_dump custom format, 14-dump retention)
prod-backup *args:
    set -a; . ./.env; set +a
    @scripts/backup.sh deploy/docker-compose.prod.yml ./backups {{ args }}

# Prove the newest backup restores (throwaway postgres, prod untouched)
prod-restore-drill:
    @scripts/restore-drill.sh
