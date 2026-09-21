# FitCore

FitCore is a gym management platform: multi-branch locations, members and
memberships, class scheduling with bookings and attendance, and invoicing.
It is built as a hexagonal Go service with a PostgreSQL (PostGIS) database,
GORM-persisted repositories, and a Prometheus/Grafana observability stack.

## Layout

```
cmd/server       HTTP API server entrypoint
cmd/migrate      migration CLI (up/down/version/create)
internal/config  environment-driven configuration
internal/httpapi Gin router and HTTP middleware
internal/modules Business modules, one package per bounded context
internal/platform Infrastructure platforms (postgres, httpx, logging, telemetry)
migrations       golang-migrate SQL migrations
deploy           docker-compose plus Prometheus/Grafana/Alertmanager config
```

## Quick start

Requires Go 1.27 and Docker.

```sh
cp .env.example .env
just compose-up       # builds and starts postgres, pgbouncer, server, observability
just migrate-up       # apply SQL migrations manually
curl http://localhost:8080/healthz
```

Migrations are never applied automatically; run `just migrate-up` (or the
`cmd/migrate` binary) explicitly. Grafana is on :3000 (admin/admin), Prometheus
on :9090, Alertmanager on :9093.

## Local development

```sh
just run                # DATABASE_URL must point at a running postgres
just test               # unit tests
just check              # full pipeline: fmt, vet, lint, build, test, spec drift
just gen-api            # regenerate openapi.gen.go from api/openapi.yaml
just install-hooks      # register .githooks/pre-commit (runs `just check`)
just migrate-up         # apply pending SQL migrations
just migrate-create name=add_cool_feature
```

Generated code (`internal/httpapi/openapi/openapi.gen.go`) is committed; after
changing the spec run `just gen-api` and implement the affected adapter.

## API documentation

The API is contract-first: an OpenAPI 3.0.3 spec in `api/openapi.yaml` documents
the full planned surface (branches, members, memberships, packages, classes,
bookings, attendance, invoices, staff, trainers). The spec is the single source
of truth — HTTP handler types and models are generated from it with
[oapi-codegen](https://github.com/oapi-codegen/oapi-codegen) (`just gen-api`).
The spec is embedded in the binary and served by the API server:

- `http://localhost:8080/openapi.yaml` — the raw spec
- `http://localhost:8080/swagger/` — interactive Swagger UI

Editing a route:

1. Update `api/openapi.yaml`.
2. Run `just gen-api` to regenerate
   `internal/httpapi/openapi/openapi.gen.go`.
3. Implement/override the corresponding method in
   `internal/httpapi/handlers`.

Routes not yet implemented return `501` with `{"error":"not implemented"}`
from `internal/httpapi/handlers`; implement them incrementally behind the
generated interface.

## Quality gates

`just check` runs the full pipeline locally: formatting, `go vet`,
`golangci-lint`, build, unit tests, and a spec/generated-code drift check
(`just gen-check`). A `pre-commit` hook runs it on every commit
(`just install-hooks` sets `core.hooksPath`). CI
(`.github/workflows/ci.yml`) runs the same pipeline plus integration tests
(against a freshly migrated Postgres service) and docker image builds.

## Migration workflow

```sh
just migrate-create name=something   # creates 000002_something.up.sql/.down.sql
just migrate-up
just migrate-down
just migrate-version
```

## Integration tests

Integration tests are gated behind the `integration` build tag and need a
dedicated test database that has already been migrated.

```sh
createdb fitcore_test             # once
DATABASE_URL='postgres://fitcore:fitcore@localhost:5432/fitcore_test?sslmode=disable' \
  just migrate-up                 # migrate it manually first
TEST_DATABASE_URL='postgres://fitcore:fitcore@localhost:5432/fitcore_test?sslmode=disable' \
  just test-integration
```

`internal/testutil` only opens the database; it does not create or migrate it.

## Configuration

All configuration is read from the environment. See `.env.example`. The only
required variable is `DATABASE_URL`.

## Architecture

- Modules express domain logic through ports (Go interfaces) defined in each
  package; the HTTP adapter and the persistence adapter live at the edge.
- `internal/platform/postgres` abstracts GORM behind a `DB` handle plus
  `MigrateUp`/`MigrateDown`/`MigrateVersion`.
- `internal/platform/telemetry` owns the Prometheus registry; modules receive
  narrow recorder interfaces rather than the concrete `Metrics` type.
- `internal/httpapi` wires middleware, health/readiness probes, the `/metrics`
  endpoint, the OpenAPI docs, and the generated handlers
  (`openapi.RegisterHandlers`). Each module exposes its adapter in
  `internal/httpapi/handlers`, returning `501` until implemented.

## Observability

- Metrics: `fitcore_http_requests_total`, `fitcore_http_request_duration_seconds`,
  `fitcore_application_errors_total`, `fitcore_database_errors_total`,
  `fitcore_memberships_purchased_total` (base Go/process collectors included).
- Alerts: `deploy/prometheus/rules.yml` (5xx rate, p95 latency, DB errors).
- Dashboards: `deploy/grafana/dashboards/fitcore.json` auto-provisioned.
- Alertmanager delivers to `ALERTMANAGER_WEBHOOK_URL` when set.
