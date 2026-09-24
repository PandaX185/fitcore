#!/usr/bin/env bash
# members_flow.sh — exercises every members endpoint against the real API:
#   list, create, get, patch, delete, malformed-ID, not-found, validation,
#   duplicate-email, and 403 negative cases (a viewer with no member grants).
#
# Expectations encode the contract from api/openapi.yaml + service rules:
#   GET    /members            200 (list, members:read)
#   POST   /members            201 (create) / 400 (invalid input: nil branch,
#                              blank name, blank/@-less email) / 409 (dup email)
#   GET    /members/{id}       200 / 400 (malformed or nil UUID) / 404 (missing)
#   PATCH  /members/{id}       200 / 404 / 409 (dup email)
#   DELETE /members/{id}       204 / 404
#   POST   /members            403 for a token without members:create
#   GET    /members            403 for a token without members:read
#
# Run: scripts/smoke/members_flow.sh   (SMOKE_VIEWER_PASSWORD overridable)
set -uo pipefail
SELF_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib.sh
source "$SELF_DIR/lib.sh"

: "${SMOKE_EMAIL:=admin@fitcore.local}"
: "${SMOKE_PASSWORD:=Password1!}"
: "${SMOKE_VIEWER:=viewer@fitcore.local}"
: "${SMOKE_VIEWER_PASSWORD:=Password1!}"

preflight

echo "== members flow =="

# 1. Admin login (seeded by the smoke recipe with member grants).
login "$SMOKE_EMAIL" "$SMOKE_PASSWORD"

# 2. List members (may be empty or carry rows from previous runs).
req GET /members 200 "" -H "Authorization: Bearer $ACCESS_TOKEN"

# 3. Create a branch to own the member (members.branch_id is a real FK).
BR_NAME="SmokeMemberBranch-$(date +%s)"
req POST /branches 201 "$(python3 -c 'import json,sys
print(json.dumps({"name": sys.argv[1], "address": "1 Member St",
                    "latitude": 31.5, "longitude": -8.0}))' "$BR_NAME")" \
    -H "Authorization: Bearer $ACCESS_TOKEN"
BR_ID=$(printf '%s' "$SMOKE_BODY" | json_get id)

# 4. Create a member; the unique email makes the duplicate-email case reliable.
MB_NAME="SmokeMember-$(date +%s)"
MB_EMAIL="$(date +%s%N)@smoke.local"
req POST /members 201 "$(python3 -c 'import json,sys
print(json.dumps({"branchId": sys.argv[1], "name": sys.argv[2],
                    "email": sys.argv[3], "phone": "5550100"}))' "$BR_ID" "$MB_NAME" "$MB_EMAIL")" \
    -H "Authorization: Bearer $ACCESS_TOKEN"
MB_ID=$(printf '%s' "$SMOKE_BODY" | json_get id)

# 5. Fetch by ID; malformed and nil UUIDs are 400, a missing row is 404.
if [[ -n "$MB_ID" ]]; then
    req GET "/members/$MB_ID" 200 "" -H "Authorization: Bearer $ACCESS_TOKEN"
fi
req GET /members/not-a-uuid 400 "" -H "Authorization: Bearer $ACCESS_TOKEN"
req GET "/members/00000000-0000-0000-0000-000000000000" 400 "" -H "Authorization: Bearer $ACCESS_TOKEN"
req GET "/members/11111111-1111-1111-1111-111111111111" 404 "" -H "Authorization: Bearer $ACCESS_TOKEN"

# 6. Patch the member (needs members:update) and re-fetch.
if [[ -n "$MB_ID" ]]; then
    req PATCH "/members/$MB_ID" 200 '{"name":"SmokeMember-updated"}' \
        -H "Authorization: Bearer $ACCESS_TOKEN"
    req GET "/members/$MB_ID" 200 "" -H "Authorization: Bearer $ACCESS_TOKEN"
fi

# 7. Validation and conflict cases: nil branch → 400, blank name → 400,
#    blank email → 400, @-less email → 400, bad body → 400, duplicate → 409.
req POST /members 400 '{"branchId":"00000000-0000-0000-0000-000000000000","name":"Ada","email":"a@b.com"}' \
    -H "Authorization: Bearer $ACCESS_TOKEN"
req POST /members 400 '{"branchId":"00000000-0000-0000-0000-000000000000","name":"","email":"a@b.com"}' \
    -H "Authorization: Bearer $ACCESS_TOKEN"
req POST /members 400 '{"branchId":"00000000-0000-0000-0000-000000000000","name":"Ada","email":""}' \
    -H "Authorization: Bearer $ACCESS_TOKEN"
req POST /members 400 '{"branchId":"00000000-0000-0000-0000-000000000000","name":"Ada","email":"nope"}' \
    -H "Authorization: Bearer $ACCESS_TOKEN"
req POST /members 400 'not-json' -H "Authorization: Bearer $ACCESS_TOKEN"
req POST /members 409 "$(python3 -c 'import json,sys
print(json.dumps({"branchId": sys.argv[1], "name": "Dup",
                    "email": sys.argv[2]}))' "$BR_ID" "$MB_EMAIL")" \
    -H "Authorization: Bearer $ACCESS_TOKEN"

# 8. Delete the member, then a second delete is a 404.
if [[ -n "$MB_ID" ]]; then
    req DELETE "/members/$MB_ID" 204 "" -H "Authorization: Bearer $ACCESS_TOKEN"
    req DELETE "/members/$MB_ID" 404 "" -H "Authorization: Bearer $ACCESS_TOKEN"
fi

# 9. Negative cases — viewer has branches:read only, so members calls are 403.
login "$SMOKE_VIEWER" "$SMOKE_VIEWER_PASSWORD"
req POST /members 403 '{"branchId":"00000000-0000-0000-0000-000000000000","name":"Forbidden","email":"x@y.z"}' \
    -H "Authorization: Bearer $ACCESS_TOKEN"
req GET /members 403 "" -H "Authorization: Bearer $ACCESS_TOKEN"

summary