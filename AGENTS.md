# AGENTS.md

## Build and verify

Requires Go 1.27. The module path is `github.com/PandaX185/fitcore`.

Run these before considering a change complete:

```
go build ./...
go vet ./...
gofmt -l .        # must print nothing
go test ./...
golangci-lint run ./...   # optional, uses .golangci.yml
```

`go test ./...` covers unit tests only. Integration tests are gated behind the
`integration` build tag and need `TEST_DATABASE_URL`:

```
TEST_DATABASE_URL='postgres://fitcore:fitcore@localhost:5432/fitcore_test?sslmode=disable' \
  go test -tags integration ./...
```

Live smoke flows (`scripts/smoke/`, wired into `just smoke`) exercise the real
API + infra against a running stack. They are a permanent, evolving part of the
architecture: every new feature extends or adds a module flow. Run them with:

```
just compose-up
just migrate-up
just smoke
```

(`just smoke` seeds the admin staff account with `cmd/set-password`, then runs
health/auth flows and reports `PASS|FAIL <METHOD> <path> → <got> (want <want>)`
per endpoint, exiting nonzero on any failure.)

## Structure and conventions

- Hexagonal layout: `internal/modules/*` are business modules (types + ports +
  domain tests). `internal/platform/*` are reusable infrastructure. The HTTP
  layer (`internal/httpapi`) and persistence adapters live at the edges; module
  packages must not import them.
- Domain types and their ports live in `<module>/types.go` and `<module>/ports.go`.
- Sentinels (`ErrNotFound`, `ErrInvalid`, `ErrDuplicateEmail`) are returned by
  the domain and mapped to HTTP status codes by `internal/platform/httpx`.
- GORM is used only inside persistence adapters, never in `internal/modules`.
- Branch-style repos live in `internal/platform/postgres/<feature>_repo.go` by
  design: each one is the adapter for a module's repository port, so it sits
  with the shared DB, transaction, and connection machinery it uses, while the
  module keeps only the port interface.
- SQL migrations live in `migrations/` and are applied via the `cmd/migrate`
  binary through `internal/platform/postgres`. Never change an applied
  migration; add a new one.

## Adding a feature

Adding or changing an endpoint, OpenAPI schema, httpapi handler, domain
service, repository query, or their tests: follow the workflow in
`docs/adding-a-feature.md` before writing code.

## Dependency and git discipline

- Prefer the standard library; add dependencies only when an equivalent does
  not exist and fits the project.
- Never print or commit secrets. `.env` files are gitignored.
- Never force-push, hard-reset, rewrite history, or delete branches without
  explicit approval.
- Do not create commits unless asked.