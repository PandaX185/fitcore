#!/usr/bin/env bash
# lib.sh — shared helpers for scripts/smoke/*_flow.sh
#
# Usage (inside a flow script, after this file is sourced):
#   req <METHOD> <path> <want_code> [json_body] [-H 'Authorization: Bearer …']
#   summary   — print PASS/FAIL tallies; exit 1 if any call failed
#
# Every helper prints one stable line per call so the flow output is a
# human-scannable contract log:
#   PASS  GET /branches → 200 (want 200)
#   FAIL  GET /branches → 401 (want 200) error=unauthorized
#
# Environment:
#   SMOKE_BASE (default http://localhost:8080) — base URL of the running API
set -uo pipefail

SMOKE_BASE="${SMOKE_BASE:-http://localhost:8080}"

_pass=0
_fail=0
_failed=0
# SMOKE_BODY holds the last response body so flows can extract fields (e.g.
# a rotated refresh token) via: printf '%s' "$SMOKE_BODY" | json_get refreshToken
SMOKE_BODY=""

# preflight — confirm the API is reachable before running a flow; gives an
# actionable message instead of a wall of curl errors.
preflight() {
    local code
    if ! code=$(curl -s -o /dev/null -w '%{http_code}' --max-time 3 "${SMOKE_BASE}/healthz" 2>/dev/null); then
        echo "smoke: cannot reach ${SMOKE_BASE} — run \`just compose-up\` first, then apply migrations (just migrate-up)" >&2
        exit 1
    fi
    if [[ "$code" != "200" ]]; then
        echo "smoke: ${SMOKE_BASE}/healthz returned ${code}, not 200 — is the API healthy?" >&2
        exit 1
    fi
}

# usage: req METHOD PATH WANT [BODY] [-H HDR]...
req() {
    local method="$1" path="$2" want="$3" body=""
    shift 3
    local -a extra=()
    while [[ $# -gt 0 ]]; do
        case "$1" in
            -H) extra+=(-H "$2"); shift 2 ;;
            *)  body="$1"; shift ;;
        esac
    done

    local args=(-sS -o /tmp/smoke_body.$$ -w '%{http_code}' -X "$method")
    [[ -n "$body" ]] && args+=(-H 'Content-Type: application/json' -d "$body")
    [[ ${#extra[@]} -gt 0 ]] && args+=("${extra[@]}")
    args+=("${SMOKE_BASE}${path}")

    local code
    if ! code=$(curl "${args[@]}" 2>/tmp/smoke_err.$$); then
        _fail=$(( _fail + 1 ))
        _failed=1
        printf 'FAIL  %s %s → curl-error (want %s): %s\n' \
            "$method" "$path" "$want" "$(cat /tmp/smoke_err.$$)"
        rm -f /tmp/smoke_body.$$ /tmp/smoke_err.$$
        return 1
    fi

    SMOKE_BODY=$(cat /tmp/smoke_body.$$ 2>/dev/null || echo '')
    local err_code
    err_code=$(printf '%s' "$SMOKE_BODY" | python3 -c 'import json,sys
try:
    d=json.load(sys.stdin)
    e=d.get("error") or {}
    if isinstance(e, str):
        err = e
    elif isinstance(e, dict):
        err = e.get("code") or e.get("message") or ""
    else:
        err = ""
    print(err if isinstance(err, str) else repr(err))
except Exception:
    print("")' 2>/dev/null)

    if [[ "$code" == "$want" ]]; then
        _pass=$(( _pass + 1 ))
        printf 'PASS  %s %s → %s (want %s)\n' "$method" "$path" "$code" "$want"
        rm -f /tmp/smoke_body.$$ /tmp/smoke_err.$$
        return 0
    fi

    _fail=$(( _fail + 1 ))
    _failed=1
    printf 'FAIL  %s %s → %s (want %s)%s\n' \
        "$method" "$path" "$code" "$want" \
        "${err_code:+ error=$err_code}"
    rm -f /tmp/smoke_body.$$ /tmp/smoke_err.$$
    return 1
}

# json_get <field> — extract a top-level field from stdin JSON (python3).
json_get() {
    python3 -c 'import json,sys
try:
    v = json.load(sys.stdin).get("'"$1"'")
    print(v if v is not None else "")
except Exception:
    print("")'
}

# login <email> <password> — sets ACCESS_TOKEN and REFRESH_TOKEN, exiting the
# flow if login itself fails (a root-cause signal, not a per-endpoint one).
ACCESS_TOKEN=""
REFRESH_TOKEN=""
login() {
    local email="$1" password="$2"
    local resp
    resp=$(curl -sS -X POST -H 'Content-Type: application/json' \
        -d "$(printf '{"email":"%s","password":"%s"}' "$email" "$password")" \
        "${SMOKE_BASE}/auth/login" 2>/dev/null) || {
        echo "smoke: login request failed (is ${SMOKE_BASE} up? run: just compose-up)" >&2
        exit 1
    }
    ACCESS_TOKEN=$(printf '%s' "$resp" | json_get accessToken)
    REFRESH_TOKEN=$(printf '%s' "$resp" | json_get refreshToken)
    if [[ -z "$ACCESS_TOKEN" || -z "$REFRESH_TOKEN" ]]; then
        echo "smoke: login failed for ${email}: ${resp}" >&2
        exit 1
    fi
}

summary() {
    printf '\n%d passed, %d failed\n' "$_pass" "$_fail"
    [[ "$_fail" -eq 0 ]]
}