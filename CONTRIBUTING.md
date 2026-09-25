# Contributing to FitCore

Thanks for wanting to contribute. FitCore is a small, spec-first service, and
the repository is set up so that most of the process is enforced locally and
by CI rather than by policy. This file covers the essentials; for the full
feature-by-feature workflow see [docs/adding-a-feature.md](docs/adding-a-feature.md).

## Development setup

Prerequisites: Go 1.27, Docker, and [`just`](https://github.com/casey/just).

```sh
cp .env.example .env
git config core.hooksPath .githooks     # or: just install-hooks
just compose-up                        # postgres, pgbouncer, redis, server, observability
just migrate-up                        # apply migrations (never automatic)
just check                             # full local pipeline: fmt, vet, lint, build, test, gen-check
```

The pre-commit hook runs `just check` on every commit, so a commit that would
break the pipeline never lands.

## How a feature lands

Every feature follows the spec-first path in order:

1. `api/openapi.yaml` states the contract (request/response/error statuses).
2. `just gen-api` regenerates `internal/httpapi/openapi/openapi.gen.go` —
   never hand-edit generated files.
3. Domain rules in `internal/modules/<feature>/` (`types.go` + `ports.go` +
   `service.go`) with sentinel errors (`ErrNotFound`, `ErrInvalid`, ...).
4. Persistence in `internal/platform/postgres/<feature>_repo.go`; schema
   changes are additive migrations (`just migrate-create`).
5. HTTP adapter in `internal/httpapi/handlers/<feature>.go` mapping sentinels
   to status codes.
6. Tests written alongside: service unit tests (fake port, no DB), integration
   repo tests (`//go:build integration`), and a smoke flow.
7. Merge only when `just check`, `just test-integration`, and the smoke flows
   are green.

The details — including the checklists — are in
[docs/adding-a-feature.md](docs/adding-a-feature.md).

## Code conventions

- **Formatting**: `gofmt`. `just check` fails on drift.
- **Linting**: `golangci-lint` with `.golangci.yml`. `just check` runs it.
- **JSON**: snake_case, matching the OpenAPI spec exactly (the generated
  request/response types dictate this).
- **Module shape**: every module mirrors `internal/modules/branches` —
  `types.go` (model + sentinels), `ports.go` (repository interface),
  `service.go` (business rules). No GORM, gin, or HTTP semantics inside
  `internal/modules/`.
- **Adapters**: HTTP adapters consume services through a consumer-owned
  interface declared in the adapter package; domain packages never import
  `internal/httpapi` or `internal/platform/postgres`.
- **Errors**: services return sentinels; the adapter maps them via
  `httpx.StatusFor`. Unexpected failures go through `httpx.Error` (logged,
  metered, client-safe message — never leak internals).
- **Migration hygiene**: never edit an applied migration; add a new one.

## Testing

Run the full set against a live stack:

```sh
just test                # unit tests
just test-integration    # needs a migrated TEST_DATABASE_URL (see README)
just smoke               # every smoke flow against the running API
just report              # unit + integration + smoke, race + coverage, grouped scenarios
```

New endpoints must be covered by:

- a service unit test for each business rule;
- an integration-tagged repo test against the real schema;
- a smoke-flow assertion for the happy path **and** at least one negative case
  (e.g. a 403/401 or a validation rejection).

Every smoke step carries a `-n 'scenario label'`; add one for each new
assertion so it shows up in `just report`'s grouped scenario list. See
`scripts/smoke/lib.sh` for the helpers.

## Commit and pull requests

- Keep commits focused and self-contained; the pre-commit hook enforces the
  pipeline per commit.
- Follow conventional commit prefixes (`feat:`, `fix:`, `test:`, `docs:`,
  `refactor:`, `chore:`), lowercase, with a body explaining the *why* when it
  is not obvious.
- Before opening a PR: rebase onto `main` instead of merging, and confirm
  `just check` plus the integration suite are green locally. CI runs the same
  gates independently.
- PR description: what changed, how it was verified, and any tradeoffs or
  follow-ups.

## CI

`.github/workflows/ci.yml` runs on every push to `main` and on PRs:

- **Full pipeline** — installs `just` and `golangci-lint`, then runs
  `just check` (identical to the local gate).
- **Integration** — brings up a fresh postgres service, migrates it,
  and runs `just test-integration`.
- **Images** — builds the server and migrate Dockerfiles.

## Reporting issues

Open an issue with the shortest reproduction you can: the API/tool version
(commit), the request (method, path, body), the response you got, and the
response you expected. Bug fixes land with a regression test.