#!/usr/bin/env bash
# memberships_flow.sh — exercises the memberships endpoints against the real API:
#   create, get, list-by-member, patch, duplicate-active, not-found, validation.
#
# Expectations (from api/openapi.yaml + membership service rules):
#   POST /memberships               201 / 400 (dup active, invalid) / 404 (missing member/package)
#   GET  /memberships/{id}          200 / 400 / 404
#   GET  /members/{id}/memberships  200
#   PATCH /memberships/{id}         200 / 400 (bad status)
set -uo pipefail
SELF_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib.sh
source "$SELF_DIR/lib.sh"

: "${SMOKE_EMAIL:=admin@fitcore.local}"
: "${SMOKE_PASSWORD:=Password1!}"

preflight

echo "== memberships flow =="

login "$SMOKE_EMAIL" "$SMOKE_PASSWORD"

# 1. Branch, member and package for the membership.
BR_NAME="SmokeMemBr-$(date +%s)"
req POST /branches 201 -n 'provision branch for membership' "$(python3 -c 'import json,sys
print(json.dumps({"name": sys.argv[1], "address": "1 Mem St",
                    "latitude": 31.5, "longitude": -8.0}))' "$BR_NAME")" \
    -H "Authorization: Bearer $ACCESS_TOKEN"
BR_ID=$(printf '%s' "$SMOKE_BODY" | json_get id)
MB_ID=""
if [[ -n "$BR_ID" ]]; then
    req POST /members 201 -n 'provision member for membership' "$(python3 -c 'import json,sys
print(json.dumps({"branchId": sys.argv[1], "name": "Mem Buyer",
                    "email": sys.argv[2]}))' "$BR_ID" "$(date +%s%N)@smoke.local")" \
        -H "Authorization: Bearer $ACCESS_TOKEN"
    MB_ID=$(printf '%s' "$SMOKE_BODY" | json_get id)
fi
req POST /packages 201 -n 'provision package for membership' "$(python3 -c 'import json,sys
print(json.dumps({"name": "Pkg " + sys.argv[1], "durationDays": 30,
                    "priceCents": 25000, "currency": "BHD"}))' "$(date +%s%N)")" \
    -H "Authorization: Bearer $ACCESS_TOKEN"
PK_ID=$(printf '%s' "$SMOKE_BODY" | json_get id)

# 2. Create a membership (active, 30 days from the package's duration).
MS_ID=""
if [[ -n "$BR_ID" && -n "$MB_ID" && -n "$PK_ID" ]]; then
    req POST /memberships 201 -n 'create membership' "$(python3 -c 'import json,sys
print(json.dumps({"memberId": sys.argv[1], "packageId": sys.argv[2],
                    "branchId": sys.argv[3]}))' "$MB_ID" "$PK_ID" "$BR_ID")" \
        -H "Authorization: Bearer $ACCESS_TOKEN"
    MS_ID=$(printf '%s' "$SMOKE_BODY" | json_get id)

    # 3. Get by ID and list by member; a second purchase while the first is
    #    still active is a conflict (400 here).
    req GET "/memberships/$MS_ID" 200 -n 'fetch membership by id' "" -H "Authorization: Bearer $ACCESS_TOKEN"
    req GET "/members/$MB_ID/memberships" 200 -n 'list memberships by member' "" -H "Authorization: Bearer $ACCESS_TOKEN"
    req POST /memberships 400 -n 'duplicate active membership rejected' "$(python3 -c 'import json,sys
print(json.dumps({"memberId": sys.argv[1], "packageId": sys.argv[2],
                    "branchId": sys.argv[3]}))' "$MB_ID" "$PK_ID" "$BR_ID")" \
        -H "Authorization: Bearer $ACCESS_TOKEN"

    # 4. Patch: freeze it; then an invalid status is refused. After freezing,
    #    a fresh purchase is allowed again.
    req PATCH "/memberships/$MS_ID" 200 -n 'freeze membership' '{"status":"frozen"}' \
        -H "Authorization: Bearer $ACCESS_TOKEN"
    req PATCH "/memberships/$MS_ID" 400 -n 'invalid membership status rejected' '{"status":"garbage"}' \
        -H "Authorization: Bearer $ACCESS_TOKEN"
    req POST /memberships 201 -n 're-purchase after freeze' "$(python3 -c 'import json,sys
print(json.dumps({"memberId": sys.argv[1], "packageId": sys.argv[2],
                    "branchId": sys.argv[3]}))' "$MB_ID" "$PK_ID" "$BR_ID")" \
        -H "Authorization: Bearer $ACCESS_TOKEN"
fi

# 5. Missing and malformed lookups.
req GET "/memberships/11111111-1111-1111-1111-111111111111" 404 -n 'missing membership not found' "" -H "Authorization: Bearer $ACCESS_TOKEN"
req GET "/memberships/00000000-0000-0000-0000-000000000000" 400 -n 'nil membership uuid rejected' "" -H "Authorization: Bearer $ACCESS_TOKEN"
req POST /memberships 400 -n 'nil ids rejected' '{"memberId":"00000000-0000-0000-0000-000000000000","packageId":"00000000-0000-0000-0000-000000000000","branchId":"00000000-0000-0000-0000-000000000000"}' \
    -H "Authorization: Bearer $ACCESS_TOKEN"
if [[ -n "$BR_ID" ]]; then
    req POST /memberships 404 -n 'missing member on membership rejected' "$(python3 -c 'import json,sys
print(json.dumps({"memberId": "11111111-1111-1111-1111-111111111111",
                    "packageId": sys.argv[1], "branchId": sys.argv[2]}))' "$PK_ID" "$BR_ID")" \
        -H "Authorization: Bearer $ACCESS_TOKEN"
fi

summary