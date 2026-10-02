# FitCore load-testing

Vegeta-based harness (`scripts/load`) on an isolated Compose stack
(`deploy/docker-compose.load.yml`). It measures three things:

1. **SLO conformance** — can the stack hold a target mixed RPS under p95
   latency and error budgets?
2. **Breakpoints** — where does each endpoint class saturate?
3. **Correctness under contention** — do the capacity/race invariants hold
   when the same seats/invoices are hammered concurrently?

## The isolated stack

Own namespace `fitcore-load`, its own postgres/redis volumes — the dev stack
is untouched.

| Service | Where            |
| ------- | ---------------- |
| API     | `http://localhost:8081` |
| Postgres| postgres on `127.0.0.1:5433` |
| Redis   | internal, no host port |

`RATE_LIMIT_DISABLED=true` so load exercises real business paths rather than
the auth rate limiter (production uses the Redis-backed limiter; see
`docs/PROD.md`).

Fixture (~108k entities): 100 branches × 500 members = 50,000 members, 8,000
classes (80/branch, class `0` is each branch's contention target at your
capacity), 4 packages, 50,000 memberships (1:1 with members). Seeded by default
through batched postgres INSERTs (`just load-seed`, ~3s); pass `api` to create
it through the public API instead (validates the API path, ~100k requests, much
slower). An admin (`loadadmin@fitcore.load`, password read from stdin, never as
a flag) is seeded with every permission.

## Running it

```sh
just load-up            # build + start the isolated stack
just load-migrate       # apply schema
just load-seed-admin    # admin account
just load-seed          # seed the API fixture (capacity 30)
just load-warmup        # warm pgbouncer connections + Redis caches

just load-slo 150 5m            # SLO: p95<400ms, 5xx+transport<0.5%
just load-break                 # ramp each endpoint class to its ceiling
just load-correctness 30        # concurrency invariants (capacity, races)
just load-soak 150 45m          # 4-window drift + memory soak

just load-down          # stop (volumes kept)
just load-reset         # stop and wipe volumes — re-seed before every soak
```

Arguments are **positional** (`set positional-arguments` in the Justfile): the
recipe defaults above are `slower`'s `-rate/-duration`, not per-argument flags.

Commands are `go run` against the checkout — no binary to rebuild on change.
Reports land in `artifacts/load/report-*.json`.

## Failure semantics

- Failure = **5xx + transport errors** only. Conflict 409s (already
  checked-in, class full, duplicate invoice) and 404s (exhausted fixture
  inventory) are *correct* outcomes of concurrent stateful workflows and are
  reported separately under `client_4xx`.
- SLO: `p95 <= 400ms` and `5xx+transport <= 0.5%`, measured over the whole run.

## Results (reference VPS)

### SLO — 150 RPS, 5 minutes (the headline number)

45,000 requests, 70% reads / 20% check-in·check-out churn / 10% bookings +
invoices.

| Metric        | Value     |
| ------------- | --------- |
| Success       | 90.02%    |
| p50 / p95 / p99 | 4 / 6 / 7 ms |
| max           | 26 ms     |
| 5xx           | **0**     |
| transport errs| **0**     |
| 4xx           | 4,491 (10%) — all semantic: duplicate-invoice 409, attendance-conflict 409, exhausted-inventory 404 |

**SLO PASS.** The throughput ceiling is nowhere near: business reads run at
1,200+ RPS (p95 1 ms) while the mixed SLO mix (4:1 read:write) cruises at 150.

### Breakpoints (ramp)

| Path            | Stable ceiling                       | Notes |
| --------------- | ------------------------------------ | ----- |
| `POST /auth/login` | **≈ 40 RPS** (p95 38 ms)            | Cliff at 60 RPS: p95 13.3 s, throughput collapses to ~48 RPS. Argon2 (16384 iter / 64 MiB, par 2) saturates the CPU — this is the only real bottleneck in the stack. |
| `GET /members/:id` | ≥ 1,200 RPS (p95 1 ms)              | DB-backed read; PgBouncer + 15-conn pool comfortably flat. |
| `GET /classes`   | ≥ 1,200 RPS (p95 45 ms at ceiling)  | Query rises at the top, stays under any SLO. |
| `POST /check-ins` | ≈ 800 RPS                          | Fresh-fixture run: sub-10 ms p95, 0 server errors. Once inventory exhausts, traffic becomes instant 404/409 at any rate — the conflict path itself is cheap (µs-class). |
| `POST /bookings` | ≈ 800 RPS                          | Same profile. Capacity invariant holds (see below). |
| `POST /invoices` | ≈ 600 RPS                          | Sequential-key writes; 409 on duplicates expected at high concurrency. |

Note (bias): the check-in/booking/invoice ceilings came from a fresh fixture;
re-run ramp steps on a depleted fixture read 0 real throughput because every
request conflicts (this is the harness exhausting state, not a server limit —
latency stays µs-class throughout).

### Correctness under contention

Capacity and single-transition invariants, hammered concurrently:

| Invariant                          | Concurrency | Created | Conflicted | DB state | Pass |
| ---------------------------------- | ----------- | ------- | ---------- | -------- | ---- |
| Class capacity held at exactly-N   | 90          | 30      | 60         | 30       | ✅   |
| Check-in creates at most one record | 5           | 1       | 4          | 1        | ✅   |
| Invoice pay is a single transition | 5           | 1       | 4          | 1        | ✅   |

### Soak — 45 minutes @ 150 RPS

Four ~11-minute windows of the SLO mix. Clean full-length run (fresh fixture +
fresh token): every window p95 ≤ 6 ms with 0 5xx and 0 transport errors;
resident memory grew +28 MiB (131.9 → 160.3 MiB) and plateaued against the
512 MiB container limit — bounded GC cadence, not a leak.

Soak rerun on a depleted fixture surfaces its own harness limits rather than
server ones: exhausted inventory makes later windows ~100 % 409/404, and the
single log-in token expires mid-run (windows become 401). **0 server/transport
errors in all windows regardless** — the API itself never degrades. Always
`just load-reset && just load-seed` and use a fresh soak window before
measuring.

### What to watch

- Login is the constraint. 150 mixed RPS needs `~10 RPS` of login headroom
  (~25 % of the 40 RPS ceiling), so the tested profile leaves the Argon2
  ceiling about 3× above a realistic login burst at the SLO rate.
- Redis-backed auth limiting caps noisier login bursts in production; the SLO
  numbers above are *without* that limiter (pure business capacity).
- Memory plateau (not growth) under steady load is the leak tell; `p99` staying
  flat across soak windows is the drift tell.