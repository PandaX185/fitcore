#!/usr/bin/env bash
# packages_flow.sh — exercises every packages endpoint against the real API:
#   list, create, get, patch, malformed-id, not-found, and validation cases.
#
# Expectations (from api/openapi.yaml + package service rules):
#   GET    /packages        200 (list)
#   POST   /packages        201 (create) / 400 (invalid: bad currency, dup name)
#   GET    /packages/{id}   200 / 400 (malformed or nil UUID) / 404 (missing)
#   PATCH  /packages/{id}   200 / 400 / 404
set -uo pipefail
SELF_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib.sh
source "$SELF_DIR/lib.sh"

: "${SMOKE_EMAIL:=admin@fitcore.local}"
: "${SMOKE_PASSWORD:=Password1!}"

preflight

echo "== packages flow =="

login "$SMOKE_EMAIL" "$SMOKE_PASSWORD"

# 1. List (may include rows from previous runs).
req GET /packages 200 "" -H "Authorization: Bearer $ACCESS_TOKEN"

# 2. Create a package; duplicate names are refused, bad currency is refused.
PK_NAME="SmokePkg-$(date +%s)"
req POST /packages 201 "$(python3 -c 'import json,sys
print(json.dumps({"name": sys.argv[1], "durationDays": 30,
                    "priceCents": 25000, "currency": "bhd"}))' "$PK_NAME")" \
    -H "Authorization: Bearer $ACCESS_TOKEN"
PK_ID=$(printf '%s' "$SMOKE_BODY" | json_get id)
req POST /packages 400 "$(python3 -c 'import json,sys
print(json.dumps({"name": sys.argv[1], "durationDays": 30,
                    "priceCents": 1, "currency": "XX"}))' "$PK_NAME")" \
    -H "Authorization: Bearer $ACCESS_TOKEN"

# 3. Fetch by ID; malformed and nil UUIDs are 400, a missing row is 404.
if [[ -n "$PK_ID" ]]; then
    req GET "/packages/$PK_ID" 200 "" -H "Authorization: Bearer $ACCESS_TOKEN"
fi
req GET /packages/not-a-uuid 400 "" -H "Authorization: Bearer $ACCESS_TOKEN"
req GET "/packages/00000000-0000-0000-0000-000000000000" 400 "" -H "Authorization: Bearer $ACCESS_TOKEN"
req GET "/packages/11111111-1111-1111-1111-111111111111" 404 "" -H "Authorization: Bearer $ACCESS_TOKEN"

# 4. Patch a package and re-fetch.
if [[ -n "$PK_ID" ]]; then
    req PATCH "/packages/$PK_ID" 200 '{"priceCents": 30000}' \
        -H "Authorization: Bearer $ACCESS_TOKEN"
    req PATCH "/packages/$PK_ID" 200 '{"active": false}' \
        -H "Authorization: Bearer $ACCESS_TOKEN"
    req GET "/packages/$PK_ID" 200 "" -H "Authorization: Bearer $ACCESS_TOKEN"
fi
req PATCH "/packages/11111111-1111-1111-1111-111111111111" 404 '{"name":"Nope"}' \
    -H "Authorization: Bearer $ACCESS_TOKEN"
req PATCH "/packages/00000000-0000-0000-0000-000000000000" 400 '{"name":"Nope"}' \
    -H "Authorization: Bearer $ACCESS_TOKEN"

summary