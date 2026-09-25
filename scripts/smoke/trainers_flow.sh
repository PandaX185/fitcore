#!/usr/bin/env bash
# trainers_flow.sh — exercises the trainers endpoints against the real API:
#   create, get, list-by-branch, patch, duplicate-email, not-found.
#
# Expectations (from api/openapi.yaml + trainer service rules):
#   POST  /trainers               201 / 400 (dup email) / 404 (no branch)
#   GET   /trainers/{id}          200 / 400 / 404
#   PATCH /trainers/{id}          200 / 400 / 404
#   GET   /branches/{id}/trainers 200
set -uo pipefail
SELF_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib.sh
source "$SELF_DIR/lib.sh"

: "${SMOKE_EMAIL:=admin@fitcore.local}"
: "${SMOKE_PASSWORD:=Password1!}"

preflight

echo "== trainers flow =="

login "$SMOKE_EMAIL" "$SMOKE_PASSWORD"

# 1. Branch to attach trainers to.
BR_NAME="SmokeTrBr-$(date +%s)"
req POST /branches 201 -n 'provision branch for trainers' "$(python3 -c 'import json,sys
print(json.dumps({"name": sys.argv[1], "address": "1 Tr St",
                    "latitude": 31.5, "longitude": -8.0}))' "$BR_NAME")" \
    -H "Authorization: Bearer $ACCESS_TOKEN"
BR_ID=$(printf '%s' "$SMOKE_BODY" | json_get id)

# 2. Create a trainer.
TR_ID=""
TR_EMAIL="$(date +%s%N)@smoke.local"
if [[ -n "$BR_ID" ]]; then
    req POST /trainers 201 -n 'create trainer' "$(python3 -c 'import json,sys
print(json.dumps({"branchId": sys.argv[1], "name": "Coach Ada",
                    "email": sys.argv[2], "phone": "5550999"}))' "$BR_ID" "$TR_EMAIL")" \
        -H "Authorization: Bearer $ACCESS_TOKEN"
    TR_ID=$(printf '%s' "$SMOKE_BODY" | json_get id)

    # 3. Fetch by ID and list by branch.
    if [[ -n "$TR_ID" ]]; then
        req GET "/trainers/$TR_ID" 200 -n 'fetch trainer by id' "" -H "Authorization: Bearer $ACCESS_TOKEN"
    fi
    req GET "/branches/$BR_ID/trainers" 200 -n 'list trainers by branch' "" -H "Authorization: Bearer $ACCESS_TOKEN"

    # 4. A second trainer with the same email is refused.
    req POST /trainers 400 -n 'duplicate trainer email rejected' "$(python3 -c 'import json,sys
print(json.dumps({"branchId": sys.argv[1], "name": "Duplicate",
                    "email": sys.argv[2]}))' "$BR_ID" "$TR_EMAIL")" \
        -H "Authorization: Bearer $ACCESS_TOKEN"

    # 5. Patch: deactivate, then re-fetch.
    if [[ -n "$TR_ID" ]]; then
        req PATCH "/trainers/$TR_ID" 200 -n 'deactivate trainer' '{"active": false}' \
            -H "Authorization: Bearer $ACCESS_TOKEN"
        req GET "/trainers/$TR_ID" 200 -n 'fetch updated trainer' "" -H "Authorization: Bearer $ACCESS_TOKEN"
    fi
fi

# 6. Missing branch, missing and malformed lookups.
req POST /trainers 404 -n 'trainer on missing branch rejected' '{"branchId":"11111111-1111-1111-1111-111111111111","name":"Ghost","email":"ghost@smoke.local"}' \
    -H "Authorization: Bearer $ACCESS_TOKEN"
req GET "/trainers/11111111-1111-1111-1111-111111111111" 404 -n 'missing trainer not found' "" -H "Authorization: Bearer $ACCESS_TOKEN"
req GET "/trainers/00000000-0000-0000-0000-000000000000" 400 -n 'nil trainer uuid rejected' "" -H "Authorization: Bearer $ACCESS_TOKEN"

summary