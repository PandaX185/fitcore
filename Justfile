set positional-arguments

# FitCore development task runner.

export DATABASE_URL := env_var_or_default("DATABASE_URL", "postgres://fitcore:fitcore@localhost:5432/fitcore?sslmode=disable")
export TEST_DATABASE_URL := env_var_or_default("TEST_DATABASE_URL", "postgres://fitcore:fitcore@localhost:5432/fitcore_test?sslmode=disable")
export TEST_REDIS_URL := env_var_or_default("TEST_REDIS_URL", "redis://localhost:6379/1")

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
    docker compose -f deploy/docker-compose.yml --env-file .env up -d --build

# Stop the dev stack
compose-down:
    docker compose -f deploy/docker-compose.yml --env-file .env down

# Seed the staff accounts used by the smoke flows (and reports)
seed-smoke email='admin@fitcore.local' password='Password1!':
    @go run ./cmd/set-password -email {{ email }} -password {{ password }} -perms 'branches:read,branches:create,branches:update,members:read,members:create,members:update,members:delete,memberships:read,classes:read,staff:read,trainers:read'
    @go run ./cmd/set-password -email viewer@fitcore.local -password {{ password }} -perms 'branches:read'

# Run manual smoke flows against the live stack (scripts/smoke/): requires
# compose-up + migrations applied; seeds the auth-flow staff account on demand.
smoke email='admin@fitcore.local' password='Password1!':
    just seed-smoke {{ email }} {{ password }}
    @scripts/smoke/health_flow.sh
    @SMOKE_EMAIL={{ email }} SMOKE_PASSWORD={{ password }} scripts/smoke/auth_flow.sh
    @scripts/smoke/branches_flow.sh
    @scripts/smoke/members_flow.sh

# Produce a full test report (scripts/report-tests.sh): unit+integration tests
# with coverage, race detection, pass/fail counts + reasons, then the smoke
# flows. Exit nonzero on any failure. Requires compose-up + migrations applied
# and the API running (just run). Options: --export [DIR] writes the report to
# DIR/test-report-<timestamp>.txt (DIR defaults to artifacts).
report *args:
    @scripts/report-tests.sh {{ args }}
