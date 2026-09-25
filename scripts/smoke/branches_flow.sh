#!/usr/bin/env bash
# branches_flow.sh — exercises every branches endpoint against the real API:
#   list, create, get, patch, malformed-ID, not-found, validation, and the 403
#   negative case (a viewer with branches:read but no branches:create).
#
# Expectations encode the contract from api/openapi.yaml + service rules:
#   GET    /branches           200 (list, branches:read)
#   POST   /branches           201 (create) / 400 (bad body) / 422 (empty
#                              name or out-of-range coords)
#   GET    /branches/{id}      200 / 400 (malformed UUID) / 404 (missing)
#   PATCH  /branches/{id}      200 / 404
#   POST   /branches           403 for a token without branches:create
#
# Run: scripts/smoke/branches_flow.sh   (SMOKE_VIEWER_PASSWORD overridable)
set -uo pipefail
SELF_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib.sh
source "$SELF_DIR/lib.sh"

: "${SMOKE_EMAIL:=admin@fitcore.local}"
: "${SMOKE_PASSWORD:=Password1!}"
: "${SMOKE_VIEWER:=viewer@fitcore.local}"
: "${SMOKE_VIEWER_PASSWORD:=Password1!}"

preflight

echo "== branches flow =="

# 1. Admin login (seeded by the smoke recipe with branches grants).
login "$SMOKE_EMAIL" "$SMOKE_PASSWORD"

# 2. List branches (may be empty or already populated across runs).
req GET /branches 200 -n 'list branches' "" -H "Authorization: Bearer $ACCESS_TOKEN"

# 3. Create a branch with a distinct name so list/get/patch have a target.
BR_NAME="SmokeBranch-$(date +%s)"
req POST /branches 201 -n 'create branch' "$(python3 -c 'import json,sys
print(json.dumps({"name": sys.argv[1], "address": "1 Flow St",
                    "latitude": 31.5, "longitude": -8.0}))' "$BR_NAME")" \
    -H "Authorization: Bearer $ACCESS_TOKEN"
BR_ID=$(printf '%s' "$SMOKE_BODY" | json_get id)

# 4. Fetch the created branch by ID; also prove malformed IDs, the nil-UUID
#    validation branch (422), and a real not-found (404).
if [[ -n "$BR_ID" ]]; then
    req GET "/branches/$BR_ID" 200 -n 'fetch branch by id' "" -H "Authorization: Bearer $ACCESS_TOKEN"
fi
req GET /branches/not-a-uuid 400 -n 'malformed branch id rejected' "" -H "Authorization: Bearer $ACCESS_TOKEN"
req GET "/branches/00000000-0000-0000-0000-000000000000" 422 -n 'nil branch uuid rejected' "" -H "Authorization: Bearer $ACCESS_TOKEN"
req GET "/branches/11111111-1111-1111-1111-111111111111" 404 -n 'missing branch not found' "" -H "Authorization: Bearer $ACCESS_TOKEN"

# 5. Patch the branch (needs branches:update in the admin grant) and re-fetch.
if [[ -n "$BR_ID" ]]; then
    req PATCH "/branches/$BR_ID" 200 -n 'update branch' '{"name":"SmokeBranch-updated"}' \
        -H "Authorization: Bearer $ACCESS_TOKEN"
    req GET "/branches/$BR_ID" 200 -n 'fetch updated branch' "" -H "Authorization: Bearer $ACCESS_TOKEN"
fi

# 6. Validation branches: empty name → 422, bad body → 400, out-of-range coord → 422.
req POST /branches 422 -n 'empty branch name rejected' '{"name":"","address":"x","latitude":0,"longitude":0}' \
    -H "Authorization: Bearer $ACCESS_TOKEN"
req POST /branches 400 -n 'malformed branch body rejected' 'not-json' -H "Authorization: Bearer $ACCESS_TOKEN"
req POST /branches 422 -n 'out-of-range branch coordinates rejected' '{"name":"Bad Coord","address":"x","latitude":95,"longitude":0}' \
    -H "Authorization: Bearer $ACCESS_TOKEN"

# 7. Negative case — viewer has branches:read only, so create must be 403.
login "$SMOKE_VIEWER" "$SMOKE_VIEWER_PASSWORD"
req POST /branches 403 -n 'create denied without branches:create' '{"name":"Forbidden","address":"x","latitude":0,"longitude":0}' \
    -H "Authorization: Bearer $ACCESS_TOKEN"

summary