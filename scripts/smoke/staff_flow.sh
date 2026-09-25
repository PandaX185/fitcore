#!/usr/bin/env bash
# staff_flow.sh — exercises the staff endpoints against the real API:
#   create, get, list-by-branch, patch, duplicate-email, not-found.
#
# Expectations (from api/openapi.yaml + staff service rules):
#   POST  /staff               201 / 400 (dup email) / 404 (no branch)
#   GET   /staff/{id}          200 / 400 / 404
#   PATCH /staff/{id}          200 / 400 / 404
#   GET   /branches/{id}/staff 200
set -uo pipefail
SELF_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib.sh
source "$SELF_DIR/lib.sh"

: "${SMOKE_EMAIL:=admin@fitcore.local}"
: "${SMOKE_PASSWORD:=Password1!}"

preflight

echo "== staff flow =="

login "$SMOKE_EMAIL" "$SMOKE_PASSWORD"

# 1. Branch to attach staff to.
BR_NAME="SmokeStaffBr-$(date +%s)"
req POST /branches 201 -n 'provision branch for staff' "$(python3 -c 'import json,sys
print(json.dumps({"name": sys.argv[1], "address": "1 Staff St",
                    "latitude": 31.5, "longitude": -8.0}))' "$BR_NAME")" \
    -H "Authorization: Bearer $ACCESS_TOKEN"
BR_ID=$(printf '%s' "$SMOKE_BODY" | json_get id)

# 2. Create a staff member with a grant.
STF_ID=""
STF_EMAIL="$(date +%s%N)@smoke.local"
if [[ -n "$BR_ID" ]]; then
    req POST /staff 201 -n 'create staff member' "$(python3 -c 'import json,sys
print(json.dumps({"branchId": sys.argv[1], "name": "Desk Agent",
                    "email": sys.argv[2], "phone": "5550123",
                    "permissions": ["branches:read", "members:read"]}))' "$BR_ID" "$STF_EMAIL")" \
        -H "Authorization: Bearer $ACCESS_TOKEN"
    STF_ID=$(printf '%s' "$SMOKE_BODY" | json_get id)

    # 3. Fetch by ID and list by branch.
    if [[ -n "$STF_ID" ]]; then
        req GET "/staff/$STF_ID" 200 -n 'fetch staff by id' "" -H "Authorization: Bearer $ACCESS_TOKEN"
    fi
    req GET "/branches/$BR_ID/staff" 200 -n 'list staff by branch' "" -H "Authorization: Bearer $ACCESS_TOKEN"

    # 4. A second account with the same email is refused.
    req POST /staff 400 -n 'duplicate staff email rejected' "$(python3 -c 'import json,sys
print(json.dumps({"branchId": sys.argv[1], "name": "Duplicate",
                    "email": sys.argv[2]}))' "$BR_ID" "$STF_EMAIL")" \
        -H "Authorization: Bearer $ACCESS_TOKEN"

    # 5. Patch: deactivate, then re-fetch.
    if [[ -n "$STF_ID" ]]; then
        req PATCH "/staff/$STF_ID" 200 -n 'deactivate staff member' '{"active": false}' \
            -H "Authorization: Bearer $ACCESS_TOKEN"
        req GET "/staff/$STF_ID" 200 -n 'fetch updated staff' "" -H "Authorization: Bearer $ACCESS_TOKEN"
    fi
fi

# 6. Missing branch, missing and malformed lookups.
req POST /staff 404 -n 'staff on missing branch rejected' '{"branchId":"11111111-1111-1111-1111-111111111111","name":"Ghost","email":"ghost@smoke.local"}' \
    -H "Authorization: Bearer $ACCESS_TOKEN"
req GET "/staff/11111111-1111-1111-1111-111111111111" 404 -n 'missing staff not found' "" -H "Authorization: Bearer $ACCESS_TOKEN"
req GET "/staff/00000000-0000-0000-0000-000000000000" 400 -n 'nil staff uuid rejected' "" -H "Authorization: Bearer $ACCESS_TOKEN"

summary