# Adding a feature

The API is spec-first: `api/openapi.yaml` is the source of truth. A feature
descends the stack: spec → generated code → domain → persistence → HTTP adapter
→ wiring. Add a new feature only by this path, in order. Each step ends with a
completion criterion; do not surface a step complete until its criterion holds.

Every module follows the `internal/modules/branches` shape: `types.go` (model +
sentinels) and `ports.go` (repository interface). The first feature also creates
three things the scaffold intentionally lacks: a `service.go`, a postgres
repository, and real dependencies in the HTTP adapter.

## Step 0 — Place the change

Decide whether the feature fits an existing module (`internal/modules/<x>`) or
starts a new one. Existing modules: attendance, billing, bookings, branches,
classes, members, memberships, packages, staff, trainers.

- [ ] You can name the module and every file the change will touch.

## Step 1 — Spec first (`api/openapi.yaml`)

Add the `paths` operation and its `components/schemas`. Match existing
conventions: snake_case JSON names, `format: uuid` on IDs, `$ref` to schemas,
`nullable: true` (never `oneOf` + null), and every non-2xx status you rely on in
`responses`. The spec is the contract; the generated code and the live docs
(`/openapi.yaml`, `/swagger/`) follow it.

- [ ] `python3 -c "import yaml; yaml.safe_load(open('api/openapi.yaml'))"` parses,
      and the spec states the request, response, and error contract exactly.

## Step 2 — Regenerate (`just gen-api`)

Regenerates `internal/httpapi/openapi/openapi.gen.go`: the `ServerInterface`
method with its typed `oapi.*ID` / `oapi.*Params` arguments, the request/response
models, and the route binding. Never hand-edit generated files; `just gen-check`
is part of the gate for a reason.

- [ ] `go build ./...` compiles and the new method is in the generated
      `ServerInterface`.

## Step 3 — Domain (`internal/modules/<feature>/`)

Pure Go: no GORM, no gin, no HTTP semantics.

- `types.go` — exported model fields (JSON mapping lives in the adapter) and
  the sentinel errors your adapter must map (`ErrNotFound`, `ErrInvalid`,
  `ErrDuplicateEmail`...).
- `ports.go` — the repository interface this module consumes.
- `service.go` — new file, created by the first feature. Business rules live
  here, not in the handler or the repo; its constructor takes the port, and its
  methods return the sentinels.

- [ ] The package compiles and every rule you care about is drivable from a
      unit test through the port — no database needed.

## Step 4 — Persistence (`internal/platform/postgres/`)

GORM lives only here. Repos live in `internal/platform/postgres/` by design:
each `<feature>_repo.go` is the persistence adapter for a module's repository
port, so it is co-located with the shared DB, transaction, and query
infrastructure it depends on, while the module keeps only the port interface.

- Add `<feature>_repo.go` implementing the module's port. Base is
  `postgres.DB.Gorm()`. For multi-step writes use
  `postgres.NewTransactionManager(conn).WithinTransaction(ctx, fn)` and resolve
  the tx inside repo calls with `postgres.FromContext(ctx, base)`.
- Map `gorm.ErrRecordNotFound` to the module's `ErrNotFound`.
- Schema change? Add `migrations/NNNNN_<name>.up.sql` / `.down.sql` (never edit
  an applied migration) and apply with `just migrate-up`. Keep `updated_at` /
  `UpdatedAt` consistent.

- [ ] An integration-tagged store test against `TEST_DATABASE_URL` proves each
      repo method against the real schema.

## Step 5 — HTTP adapter (`internal/httpapi/handlers/<feature>.go`)

Replace the 501 stub. The generated wrapper already parsed and validated path
params (a malformed UUID is a 400 before your method runs).

- Bind the request body, call the service.
- The adapter consumes the service through a **consumer-owned interface**
  (e.g. `branchService`) declared in the adapter package with only the methods
  the endpoint needs; the module's concrete `Service` satisfies it implicitly.
  Keep it unexported. Promote it into the module (as an exported `XService`
  interface) only when a second consumer appears. `httpapi.Deps` / the adapter
  must import domain types, never the other way around.
- Map sentinels with `httpx.StatusFor(err, ErrNotFound, ErrInvalid, ErrConflict)`
  → 404 / 422 / 409; respond with generated response types via `httpx.JSON`.
- Unexpected errors go through `httpx.Error(c, logger, metrics, module, op,
  status, clientSafeMessage, err)` — it logs the cause, records the metric, and
  never leaks the cause to the client.
- The first feature wires dependencies: give `handlers.New(...)` the service
  (and logger/metrics), and add the fields to `httpapi.Deps` in `router.go`,
  passing them in `New(deps)`.

- [ ] Curl returns the right status and body on the happy path and on every
      error branch — no surprise 500s.

## Step 6 — Tests (written alongside the code)

- Service: table-driven unit tests in the module with a fake port — no DB.
- Store: `//go:build integration` test files using `internal/testutil.DB(t)`;
  needs a migrated `TEST_DATABASE_URL` (migrate it first — see AGENTS.md
  "Build and verify").
- Handler: `httptest` + gin over the port interface (mocked service), assert
  JSON shape and status.

- [ ] `go test ./...` is green, then
      `TEST_DATABASE_URL=... go test -tags integration ./...` is green.

## Step 7 — Verify and commit

```
just check
```

(fmt → vet → lint → build → test → gen-check). Then smoke-test the running
server against compose Postgres and curl the new route. The pre-commit hook
re-runs `just check`; CI mirrors it and adds integration tests.

- [ ] `just check` is green and the smoke test shows the new route, the spec
      and Swagger UI behaving.

## Done

The feature is complete when: spec is the contract, generated code matches it,
domain rules are under unit test, the schema is migrated, the adapter maps
errors cleanly, and `just check` stays green. Leftover stubs in unchanged
modules keep returning 501 — that is the intended state until their features
land.