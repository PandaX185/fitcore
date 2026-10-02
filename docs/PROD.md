# FitCore production runbook

Single-VPS deployment. Everything the API needs runs in one Compose project
(`deploy/docker-compose.prod.yml`) behind a Caddy TLS edge.

## Topology

```
internet ── 80/443 ── caddy ── server:8080 ── pgbouncer ── postgres
                         │        │
                         │        └─ redis (AOF on; revocations + auth rate limit)
                         └─ (loopback only) grafana :3000 / alertmanager :9093
Every backend+observability service sits on an internal, non-routable network.
Only caddy publishes ports; only graphana/alertmanager are operator loopback UIs.
```

- `migrate` is a one-shot service; it runs DDL straight against postgres
  (not through the pooler) and exits.
- The app pool is capped at 15 connections; pgbouncer `DEFAULT_POOL_SIZE` is
  20 under `POOL_MODE=session` — **never raise the app pool above the pooler**.
- Auth rate limiting is Redis-backed and shared: `RATE_LIMIT_REDIS=true`.
  Never set `RATE_LIMIT_DISABLED` in production.

## First deployment

1. Point a DNS A/AAAA record at the VPS for your API hostname.
2. Create the env file and secrets:
   ```sh
   cp deploy/.env.prod.example .env
   scripts/gen-secrets.sh .env   # fills POSTGRES_PASSWORD / TOKEN_SECRET / GRAFANA_ADMIN_PASSWORD
   vi .env                        # fit FITCORE_DOMAIN + CADDY_EMAIL
   chmod 600 .env
   ```
3. Open the firewall: 80/443 public; 22 restricted. (UFW example below.)
4. Start and migrate:
   ```sh
   just prod-up          # builds, starts everything, runs caddy's TLS handshake
   just prod-migrate     # apply schema (idempotent one-shot)
   ```
   The server waits for pgbouncer/postgres/redis to be healthy before starting,
   so there is no DNS startup race.

## Day-2 operations (Justfile)

| Task            | Command                    |
| --------------- | -------------------------- |
| Upgrade         | `just prod-up`             |
| Apply migrations| `just prod-migrate`        |
| Status          | `just prod-status`         |
| Logs            | `just prod-logs [svc]`     |
| Stop            | `just prod-down`           |
| Nightly backup  | `just prod-backup`         |
| Restore drill   | `just prod-restore-drill`  |

## Secrets and rotation

Every required secret is validated by compose (`${VAR:?…}`); the stack refuses
to start with a missing value. `scripts/gen-secrets.sh` generates URL-safe hex
values and the compose file fails closed rather than defaulting anything
security-relevant.

Rotating `TOKEN_SECRET`:
1. `just prod-down`
2. Generate a new secret, put it in `.env` (`TOKEN_SECRET` only).
3. `just prod-up` — all old JWTs become invalid immediately (HMAC mismatch).
   Sessions are out until users re-login, which also re-issues refresh tokens.

Redis holds revocations with TTLs; AOF persistence (`--appendonly yes
--appendfsync everysec`) means a container restart does not silently accept
previously-revoked tokens. Do not turn AOF off.

## Backups and recovery

Nightly `pg_dump -Fc` custom-format:
```sh
just prod-backup         # writes ./backups/fitcore-<ts>.dump, keeps 14 dumps
```
Optional off-site copy via `BACKUP_RCLONE_REMOTE="remote:dir"` (rclone on the
host): `just prod-backup` appends the copy step automatically.

RPO/RTO:
- RPO: ≤ 24h with nightly dumps alone. For point-in-time recovery, enable WAL
  archiving on postgres and take base backups (`pg_basebackup`) — schedule a
  nightly custom-format dump **plus** continuous WAL shipping to a second disk.
- RTO: a full restore of the newest dump runs in minutes (started by
  `scripts/restore-drill.sh` on a disposable container).

Drill monthly:
```sh
just prod-restore-drill   # spins a scratch postgres, restores latest dump,
                          # verifies schema + data, tears down. Prod untouched.
```
The drill is a good pre-deploy habit: run it before the first `prod-up` too.

## Observability and alerting

- Prometheus scrapes `server:8080/metrics`, redis-exporter, postgres-exporter
  and pgbouncer-exporter (30d retention in `prometheus_data`).
- Dashboards are provisioned into Grafana (loopback `:3000`).
- Alerts (`deploy/prometheus/rules.yml`) ship to Alertmanager; set
  `ALERTMANAGER_WEBHOOK_URL` in `.env` to deliver them (Slack/Teams/…).
  With no webhook, alerts stay local (Alertmanager UI at loopback `:9093`).
- App metrics include DB pool saturation (`fitcore_db_pool_open`,
  `fitcore_db_pool_in_use`) and request/error counters. Watch:
  - `fitcore_db_pool_in_use` reaching `fitcore_db_pool_open` ⇒ pool is
    saturating; raise pgbouncer `DEFAULT_POOL_SIZE` **and** the app pool
    together, or scale the service.
  - `fitcore_http_requests_total{status=~"5.."}` growth ⇒ backend fault.

## Load-test results (what the stack is sized for)

`docs/LOADTEST.md` describes the whole harness. Headline numbers from the
reference VPS:

| Scenario            | Sustained ceiling                      | Notes                |
| ------------------- | -------------------------------------- | -------------------- |
| Mixed production mix| **150 RPS**, p95 ≈ 6 ms, 0 5xx        | SLO target PASS      |
| Business reads      | ≥ 1200 RPS, p95 ≈ 1 ms                 | list-classes p95 45ms at the max boundary |
| Check-in / booking  | 600–800 RPS, sub-ms p95, 0 server errs | transactional; capacity invariants hold |
| Auth login (Argon2) | **≈ 40 RPS** sustained                 | the only real bottleneck; CPU-bound |

Runtime footprint: server ≈ 160 MiB peak under load (limit 512 MiB). The
auth/login ceiling is CPU-bound on the VPS; headroom for login bursts is gained
by giving the server more CPU or lowering Argon2 cost, not by raising pool
sizes.

## Capacity and sizing guide

- A machines' nominal traffic is bursty at class start/end (check-in waves).
  Use the SLO run (`just load-slo` against a pre-prod stack) at your expected
  peak RPS to choose VPS size.
- Keep app pool ≤ 15 and pgbouncer pool 20 unless they scale together.
- `cpus: 2` + `mem_limit: 512m` on server is the tested profile; raise CPU
  first when login latencies climb (it is Argon2-bound).

## Security checklist

- [ ] Only `80`/`443` open publicly; everything else loopback/private.
- [ ] `.env` is chmod 600, never committed, never shared; secrets fail closed.
- [ ] `security_opt: no-new-privileges` + `cap_drop: [ALL]` on app containers.
- [ ] Server image runs as non-root (`USER fitcore`), read-only-tunable.
- [ ] Redis AOF on (revocations survive restarts).
- [ ] CI scans images (Trivy) and dependencies (govulncheck); images are
      digest-pinned `ghcr.io` artifacts, pulled by tag from the registry.
- [ ] Monthly restore drill passes.
- [ ] Grafana admin password is not the default; UI is loopback-only.