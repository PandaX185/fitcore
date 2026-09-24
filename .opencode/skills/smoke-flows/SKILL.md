---
name: smoke-flows
description: Use when running, writing, extending, or debugging the live API smoke flows in scripts/smoke/ or the just smoke recipe — adding a per-endpoint check, pinning down a PASS/FAIL line, seeding staff, or wiring a new module flow into the recipe. Trigger keywords include smoke, PASS/FAIL, scripts/smoke, and just smoke.
---

# Smoke Flows

Manual testing flows against the **real running API + infra** (compose
postgres/redis/server), one flow per module, every endpoint exercised with
proper auth. This is a permanent, evolving layer: each new feature extends or
adds a module flow. Shared helpers + contract live here.

## Layout

```
scripts/smoke/
  lib.sh               # helpers: preflight, req, login, json_get, summary
  health_flow.sh       # public endpoints (/healthz, /readyz)
  auth_flow.sh         # full auth module lifecycle (login/refresh/logout)
  <module>_flow.sh     # one per module, added with each feature
Justfile               # `smoke` recipe: seeds admin, runs every flow
```

## Run

```
just compose-up
just migrate-up        # schema (v3) applied to the fitcore DB
just smoke             # seeds staff via cmd/set-password, runs all flows
```

Requirements before a run: stack up (`just compose-up`), migrations applied,
API reachable. Override target/host with `SMOKE_BASE` (default
`http://localhost:8080`), credentials with `SMOKE_EMAIL`/`SMOKE_PASSWORD`.

## Output contract

Every endpoint call prints one stable line; a FAIL line pinpoints the error and
the final exit code is nonzero iff any call failed:

```
PASS  GET /healthz → 200 (want 200)
FAIL  POST /auth/login → 400 (want 200) error=invalid request body
```

## lib.sh helpers

- `preflight` — assert /healthz is 200, else exit 1 with an actionable message
  (used at the top of every flow).
- `req METHOD PATH WANT [BODY] [-H "Header: value"]` — one HTTP call; prints
  the PASS/FAIL line; on FAIL records `error=<code|message>` from
  `{"error": {...}}`. Populates `SMOKE_BODY` with the response body.
- `login EMAIL PASSWORD` — sets `ACCESS_TOKEN` / `REFRESH_TOKEN`; exits 1 with
  root cause if login itself fails.
- `json_get FIELD` — extract a top-level field from JSON on stdin (python3).
- `summary` — print `N passed, M failed`; return 0 iff no failures.

## Adding or extending a flow

New module flow (`members_flow.sh`, `invoices_flow.sh`, …) — mirror the
existing structure:

```bash
#!/usr/bin/env bash
# <module>_flow.sh — every <module> endpoint, proper auth, one negative case each.
set -uo pipefail
SELF_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib.sh
source "$SELF_DIR/lib.sh"
preflight

# login as admin is provided by the just smoke recipe; token fields are global.
req GET /members 200 "" -H "Authorization: Bearer $ACCESS_TOKEN"
req POST /members 201 '{"name":"Flow Test"}' -H "Authorization: Bearer $ACCESS_TOKEN"
req GET /members/not-a-uuid 400 "" -H "Authorization: Bearer $ACCESS_TOKEN"  # negative case
summary
```

- Every endpoint of the feature must have a check; include at least one
  negative case (401/403/404) per new endpoint.
- Wire into the Justfile recipe: append
  `@SMOKE_EMAIL={{ email }} SMOKE_PASSWORD={{ password }} scripts/smoke/<module>_flow.sh`
- Endpoints and statuses come from `api/openapi.yaml` (the spec is the source
  of truth) and the `DefaultRegistry` in
  `internal/httpapi/middleware/authz.go` (which public/authed + required
  permissions). A smoke flow that contradicts the spec is a bug in the flow.

## Debugging a failing flow

- `FAIL` with `error=<message>` — the server rejected the call; read the
  message, then check the request shape against the generated handler in
  `internal/httpapi/handlers/`.
- `curl-error` — infra: is the stack up (`docker compose ps`)?
- `error=None` — response was not a JSON `{"error": ...}` shape; inspect
  `SMOKE_BODY` with `curl -i` on the raw endpoint.
- Wrong token scopes: check the `permissions` granted by `cmd/set-password`
  (the smoke recipe seeds admin with the grant set in the Justfile recipe).