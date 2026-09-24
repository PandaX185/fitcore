---
name: adding-a-feature
description: Use when adding or changing a fitcore feature or endpoint — work touching api/openapi.yaml, internal/httpapi handlers or router wiring, internal/modules/*, internal/platform/postgres repos, migrations/, or scripts/smoke flows. This is the mandatory workflow for end-to-end feature work and always ends with smoke verification, a report, and a commit.
---

# Adding a Feature

Spec-first: `api/openapi.yaml` is the source of truth. A feature descends the
stack: spec → generated code → domain → persistence → HTTP adapter → wiring →
tests → smoke → report → commit. Work only through this path, in order.
The detailed checklist lives in `docs/adding-a-feature.md` — read it first and
complete every step's criterion before marking it done.

## Workflow

1. **Place the change** — fit an existing module (`internal/modules/<x>`:
   attendance, billing, bookings, branches, classes, members, memberships,
   packages, staff, trainers) or start a new one. Name every file you will touch.
2. **Spec first** — edit `api/openapi.yaml`: snake_case JSON names, `format: uuid`
   on IDs, `$ref` to schemas, `nullable: true` (never `oneOf`+null), and every
   non-2xx status you rely on in `responses`. Verify it parses:
   `python3 -c "import yaml; yaml.safe_load(open('api/openapi.yaml'))"`.
3. **Regenerate** — `just gen-api` regenerates
   `internal/httpapi/openapi/openapi.gen.go`. Never hand-edit generated files;
   `just gen-check` enforces this at the gate.
4. **Domain** (`internal/modules/<feature>/`) — pure Go, no GORM/gin/HTTP:
   `types.go` (model + sentinels like `ErrNotFound`, `ErrInvalid`,
   `ErrDuplicateEmail`), `ports.go` (repository interface the module consumes),
   `service.go` (business rules; constructor takes the port, methods return
   sentinels).
5. **Persistence** (`internal/platform/postgres/<feature>_repo.go`) — GORM lives
   only here. Implement the module's port on `postgres.DB.Gorm()`. Multi-step
   writes use `postgres.NewTransactionManager(conn).WithinTransaction` +
   `postgres.FromContext`. Map `gorm.ErrRecordNotFound` → the module's
   `ErrNotFound`. Schema change? New `migrations/NNNNN_<name>.up|down.sql`
   (never edit an applied migration), then `just migrate-up`.
6. **HTTP adapter** (`internal/httpapi/handlers/<feature>.go`) — replace the 501
   stub. Bind the body, call the service through a **consumer-owned unexported
   interface** (e.g. `branchService`) declared in the adapter with only the
   methods the endpoint needs. Map sentinels via
   `httpx.StatusFor(err, ErrNotFound, ErrInvalid, ErrConflict)` → 404/422/409;
   respond via `httpx.JSON`. Unexpected errors go through `httpx.Error(...)`
   (logs cause, records metric, never leaks cause). Wire deps in
   `handlers.New(...)` + `httpapi.Deps` in `router.go`.
7. **Tests written alongside the code** — unit: table-driven service tests with a
   fake port (no DB). Store: `//go:build integration` using `internal/testutil.DB`.
   Handler: `httptest` + gin over the port (mocked service), assert JSON shape
   and status.

## Verify (gate — do not skip)

```
just check          # fmt → vet → lint → build → test → gen-check, must be green
just compose-up && just migrate-up
just smoke          # live stack flows must be PASS, exit 0
```

- **Extend or add a smoke flow** in `scripts/smoke/` covering every endpoint of
  this feature with proper auth — per-endpoint
  `PASS|FAIL <METHOD> <path> → <got> (want <want>)`, error pinpointed on FAIL,
  at least one negative case (401/403), nonzero exit on any FAIL. Wire the flow
  into the `smoke` Justfile recipe. A feature without a flow is not done.
- Output contract is defined in the `smoke-flows` skill — use it when writing
  or debugging flows.

## Report and commit

Finish with a **report**, then **commit** (this skill's workflow ends with a
commit — that is the intended behavior):

1. Report — concise end-of-work summary stating: files changed per stack layer,
   what was verified, and the exact verification evidence (commands + results
   like "just smoke: 12 passed, 0 failed"). Do not claim verification you did
   not run.
2. Commit — stage only intended files, no secrets. Conventional Commits style
   matching repo history: `feat:`, `fix:`, `docs:`, `chore:` with a lowercase
   subject. Verify with `git status` + `git diff --stat` before committing.