# FitCore

FitCore is a gym management platform: multi-branch locations, members and
memberships, package catalog, class scheduling with bookings and attendance,
and invoicing. It is built as a hexagonal Go service with a PostgreSQL
(PostGIS) database, GORM-persisted repositories, JWT bearer auth with token
revocation, and a Prometheus/Grafana observability stack.

## Features

- **Auth** — JWT login, rotating single-use refresh tokens, logout that revokes
  both access and refresh tokens, permission-gated endpoints.
- **Branches** — multi-location management (name, address, coordinates).
- **Members** — member registry per branch, duplicate-email detection, soft
  deletes.
- **Packages** — priced, duration-limited membership products.
- **Memberships** — member↔package purchase with a single *active* membership
  per member; freeze/unfreeze/re-purchase lifecycle.
- **Classes** — branch-scoped scheduling with capacity; cancel/rebook after
  cancellation is allowed.
- **Bookings** — per-member seats with duplicate and over-capacity protection.
- **Attendance** — check-in/check-out gated by an active membership.
- **Billing / Invoices** — invoicing against memberships, paid/pending/failed
  lifecycle.
- **Staff & Trainers** — per-branch staff and trainer accounts with grants.

## Technology

- **API**: OpenAPI 3.0.3 contract in `api/openapi.yaml`, server code generated
  with [oapi-codegen](https://github.com/oapi-codegen/oapi-codegen).
- **HTTP**: Gin router.
- **Persistence**: PostgreSQL/PostGIS via GORM behind repository ports.
- **Auth**: JWT bearer tokens; refresh rotation; revocation via Redis.
- **Observability**: Prometheus metrics, Grafana dashboards, Alertmanager
  alerts.

## Layout

```
cmd/server           HTTP API server entrypoint
cmd/migrate          migration CLI (up/down/version/create)
cmd/set-password     seed/update a staff account with grants (smoke helper)
internal/config      environment-driven configuration
internal/httpapi     Gin router, middleware, generated handlers
internal/modules     Business modules, one package per bounded context
internal/platform    Infrastructure platforms (postgres, httpx, logging, redis, telemetry)
migrations           golang-migrate SQL migrations
scripts/smoke        end-to-end smoke flows against a live stack
deploy               docker-compose plus Prometheus/Grafana/Alertmanager config
```

## Quick start

Requires Go 1.27 and Docker.

```sh
cp .env.example .env
just compose-up       # builds and starts postgres, pgbouncer, redis, server, observability
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
just test-integration   # integration tests (needs a migrated fitcore_test DB)
just smoke              # run every smoke flow against the live API
just report             # full report: coverage + race + smoke scenarios
just gen-api            # regenerate openapi.gen.go from api/openapi.yaml
just install-hooks      # register .githooks/pre-commit (runs `just check`)
just migrate-up         # apply pending SQL migrations
just migrate-create name=add_cool_feature
```

Generated code (`internal/httpapi/openapi/openapi.gen.go`) and the migration
SQL are committed; after changing the spec run `just gen-api` and implement the
affected adapter.

## API documentation

The API is contract-first: an OpenAPI 3.0.3 spec in `api/openapi.yaml` is the
single source of truth for every module — branches, members, memberships,
packages, classes, bookings, attendance, invoices, staff and trainers. HTTP
handler types and models are generated from it with
[oapi-codegen](https://github.com/oapi-codegen/oapi-codegen) (`just gen-api`),
so the compiled handlers always match the spec. The spec is embedded in the
binary and served by the API server:

- `http://localhost:8080/openapi.yaml` — the raw spec
- `http://localhost:8080/swagger/` — interactive Swagger UI

All endpoints defined in the spec are implemented in `internal/httpapi/handlers`
(no `501` stubs remain). Editing a route:

1. Update `api/openapi.yaml`.
2. Run `just gen-api` to regenerate
   `internal/httpapi/openapi/openapi.gen.go`.
3. Implement/override the corresponding method in
   `internal/httpapi/handlers`.

Routes are permission-gated: each requires a bearer token with the matching
`<module>:<verb>` grant (e.g. `branches:create`).

## Testing

- **Unit tests** (`just test`) — service rules and handlers via table-driven
  tests with fake ports; no database required.
- **Integration tests** (`just test-integration`) — gated behind the
  `integration` build tag; exercise every repository against a live, migrated
  `fitcore_test` database.
- **Smoke flows** (`just smoke`) — `scripts/smoke/` replays the whole API
  surface against a running stack with real auth, asserting status codes for
  happy paths and error/negative cases alike.
- **Test report** (`just report`) — runs the unit + integration suite with
  `-race` and coverage, then every smoke flow, and prints pass/fail counts,
  failed-test details, and a grouped list of the ~145 scenarios each flow
  checks (e.g. `bookings_flow → duplicate booking rejected (pass)`).
  `just report --export DIR` writes it to `DIR/test-report-<timestamp>.txt`.

See [docs/adding-a-feature.md](docs/adding-a-feature.md) for the full
spec-first workflow each feature follows.

## Migration workflow

```sh
just migrate-create name=something   # creates 000002_something.up.sql/.down.sql
just migrate-up
just migrate-down
just migrate-version
```

## Configuration

All configuration is read from the environment. See `.env.example`. The only
required variable is `DATABASE_URL`.

## Architecture

- Modules express domain logic through ports (Go interfaces) defined in each
  package; the HTTP adapter and the persistence adapter live at the edge.
- `internal/platform/postgres` abstracts GORM behind a `DB` handle plus
  `MigrateUp`/`MigrateDown`/`MigrateVersion`, and hosts one repository per
  module.
- `internal/platform/telemetry` owns the Prometheus registry; modules receive
  narrow recorder interfaces rather than the concrete `Metrics` type.
- `internal/httpapi` wires middleware, health/readiness probes, the `/metrics`
  endpoint, the OpenAPI docs, and the generated handlers
  (`openapi.RegisterHandlers`). Each module exposes its adapter in
  `internal/httpapi/handlers`; adapters consume the module service through a
  consumer-owned interface, and map service sentinels to HTTP status codes via
  `httpx.StatusFor`.

## Observability

- Metrics: `fitcore_http_requests_total`, `fitcore_http_request_duration_seconds`,
  `fitcore_application_errors_total`, `fitcore_database_errors_total`,
  `fitcore_memberships_purchased_total` (base Go/process collectors included).
- Alerts: `deploy/prometheus/rules.yml` (5xx rate, p95 latency, DB errors).
- Dashboards: `deploy/grafana/dashboards/fitcore.json` auto-provisioned.
- Alertmanager delivers to `ALERTMANAGER_WEBHOOK_URL` when set.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for development setup, conventions, and
the pull-request workflow.

## License

[MIT](LICENSE). Copyright (c) 2026 PandaX185.