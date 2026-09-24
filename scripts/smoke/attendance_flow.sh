#!/usr/bin/env bash
# attendance_flow.sh — exercises the attendance endpoints against the real API:
#   check-in, check-out, get, list-by-member, and the conflict/missing cases.
#
# Expectations (from api/openapi.yaml + attendance service rules):
#   POST /attendance/check-in        201 / 409 (already open) / 404 (no member / no membership)
#   POST /attendance/check-out       200 / 404 (no open record)
#   GET  /attendance/{id}            200 / 400 / 404
#   GET  /members/{id}/attendance    200
set -uo pipefail
SELF_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib.sh
source "$SELF_DIR/lib.sh"

: "${SMOKE_EMAIL:=admin@fitcore.local}"
: "${SMOKE_PASSWORD:=Password1!}"

preflight

echo "== attendance flow =="

login "$SMOKE_EMAIL" "$SMOKE_PASSWORD"

# 1. Branch + member with an active membership, and a member without one.
BR_NAME="SmokeAtBr-$(date +%s)"
req POST /branches 201 "$(python3 -c 'import json,sys
print(json.dumps({"name": sys.argv[1], "address": "1 At St",
                    "latitude": 31.5, "longitude": -8.0}))' "$BR_NAME")" \
    -H "Authorization: Bearer $ACCESS_TOKEN"
BR_ID=$(printf '%s' "$SMOKE_BODY" | json_get id)

MB=""
MB_NO=""
if [[ -n "$BR_ID" ]]; then
    req POST /members 201 "$(python3 -c 'import json,sys
print(json.dumps({"branchId": sys.argv[1], "name": "Visitor",
                    "email": sys.argv[2]}))' "$BR_ID" "$(date +%s%N)@smoke.local")" \
        -H "Authorization: Bearer $ACCESS_TOKEN"
    MB=$(printf '%s' "$SMOKE_BODY" | json_get id)
    req POST /members 201 "$(python3 -c 'import json,sys
print(json.dumps({"branchId": sys.argv[1], "name": "No Membership",
                    "email": sys.argv[2]}))' "$BR_ID" "$(date +%s%N)@smoke.local")" \
        -H "Authorization: Bearer $ACCESS_TOKEN"
    MB_NO=$(printf '%s' "$SMOKE_BODY" | json_get id)
fi
req POST /packages 201 "$(python3 -c 'import json,sys
print(json.dumps({"name": "Pkg " + sys.argv[1], "durationDays": 30,
                    "priceCents": 25000, "currency": "BHD"}))' "$(date +%s)")" \
    -H "Authorization: Bearer $ACCESS_TOKEN"
PK_ID=$(printf '%s' "$SMOKE_BODY" | json_get id)

# 2. Check in the member with the active membership.
AT_ID=""
if [[ -n "$MB" && -n "$PK_ID" && -n "$BR_ID" ]]; then
    req POST /memberships 201 "$(python3 -c 'import json,sys
print(json.dumps({"memberId": sys.argv[1], "packageId": sys.argv[2],
                    "branchId": sys.argv[3]}))' "$MB" "$PK_ID" "$BR_ID")" \
        -H "Authorization: Bearer $ACCESS_TOKEN"

    req POST /attendance/check-in 201 "$(python3 -c 'import json,sys
print(json.dumps({"memberId": sys.argv[1], "branchId": sys.argv[2]}))' "$MB" "$BR_ID")" \
        -H "Authorization: Bearer $ACCESS_TOKEN"
    AT_ID=$(printf '%s' "$SMOKE_BODY" | json_get id)

    # 3. A second check-in while the visit is open is a conflict.
    req POST /attendance/check-in 409 "$(python3 -c 'import json,sys
print(json.dumps({"memberId": sys.argv[1], "branchId": sys.argv[2]}))' "$MB" "$BR_ID")" \
        -H "Authorization: Bearer $ACCESS_TOKEN"

    # 4. Fetch by ID and list by member.
    if [[ -n "$AT_ID" ]]; then
        req GET "/attendance/$AT_ID" 200 "" -H "Authorization: Bearer $ACCESS_TOKEN"
    fi
    req GET "/members/$MB/attendance" 200 "" -H "Authorization: Bearer $ACCESS_TOKEN"

    # 5. Check out closes the visit; a second check-out finds nothing open.
    req POST /attendance/check-out 200 "$(python3 -c 'import json,sys
print(json.dumps({"memberId": sys.argv[1]}))' "$MB")" \
        -H "Authorization: Bearer $ACCESS_TOKEN"
    req POST /attendance/check-out 404 "$(python3 -c 'import json,sys
print(json.dumps({"memberId": sys.argv[1]}))' "$MB")" \
        -H "Authorization: Bearer $ACCESS_TOKEN"
fi

# 6. A member without an active membership cannot check in (404 here).
if [[ -n "$MB_NO" && -n "$BR_ID" ]]; then
    req POST /attendance/check-in 404 "$(python3 -c 'import json,sys
print(json.dumps({"memberId": sys.argv[1], "branchId": sys.argv[2]}))' "$MB_NO" "$BR_ID")" \
        -H "Authorization: Bearer $ACCESS_TOKEN"
fi

# 7. Missing/malformed lookups.
req GET "/attendance/11111111-1111-1111-1111-111111111111" 404 "" -H "Authorization: Bearer $ACCESS_TOKEN"
req GET "/attendance/00000000-0000-0000-0000-000000000000" 400 "" -H "Authorization: Bearer $ACCESS_TOKEN"

summary