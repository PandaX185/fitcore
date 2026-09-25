#!/usr/bin/env bash
# classes_flow.sh — exercises the classes endpoints against the real API:
#   create, list (all + branch filter), get, patch, delete, validation.
#
# Expectations (from api/openapi.yaml + class service rules):
#   GET    /classes         200 (optional branchId/trainerId filters)
#   POST   /classes         201 / 400 (ends before starts) / 404 (no branch/trainer)
#   GET    /classes/{id}    200 / 400 / 404
#   PATCH  /classes/{id}    200 / 400 / 404
#   DELETE /classes/{id}    204 / 404
set -uo pipefail
SELF_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib.sh
source "$SELF_DIR/lib.sh"

: "${SMOKE_EMAIL:=admin@fitcore.local}"
: "${SMOKE_PASSWORD:=Password1!}"

preflight

echo "== classes flow =="

login "$SMOKE_EMAIL" "$SMOKE_PASSWORD"

# 1. Branch to own the class (classes.branch_id is a real FK).
BR_NAME="SmokeClsBr-$(date +%s)"
req POST /branches 201 -n 'provision branch for class' "$(python3 -c 'import json,sys
print(json.dumps({"name": sys.argv[1], "address": "1 Cls St",
                    "latitude": 31.5, "longitude": -8.0}))' "$BR_NAME")" \
    -H "Authorization: Bearer $ACCESS_TOKEN"
BR_ID=$(printf '%s' "$SMOKE_BODY" | json_get id)

# 2. Create a class scheduled 1 day out (ISO8601 timestamps).
CL_ID=""
if [[ -n "$BR_ID" ]]; then
    STARTS="$(python3 -c 'import datetime
print((datetime.datetime.now(datetime.timezone.utc)+datetime.timedelta(days=1)).isoformat())')"
    ENDS="$(python3 -c 'import datetime
print((datetime.datetime.now(datetime.timezone.utc)+datetime.timedelta(days=1, hours=1)).isoformat())')"
    req POST /classes 201 -n 'create class' "$(python3 -c 'import json,sys
print(json.dumps({"branchId": sys.argv[1], "name": "Spin Gym",
                    "capacity": 10, "startsAt": sys.argv[2],
                    "endsAt": sys.argv[3]}))' "$BR_ID" "$STARTS" "$ENDS")" \
        -H "Authorization: Bearer $ACCESS_TOKEN"
    CL_ID=$(printf '%s' "$SMOKE_BODY" | json_get id)
fi

# 3. List all classes and filtered by branch.
req GET /classes 200 -n 'list classes' "" -H "Authorization: Bearer $ACCESS_TOKEN"
if [[ -n "$BR_ID" ]]; then
    req GET "/classes?branchId=$BR_ID" 200 -n 'list classes filtered by branch' "" -H "Authorization: Bearer $ACCESS_TOKEN"
fi

# 4. Fetch by ID; malformed and nil UUIDs are 400, missing is 404.
if [[ -n "$CL_ID" ]]; then
    req GET "/classes/$CL_ID" 200 -n 'fetch class by id' "" -H "Authorization: Bearer $ACCESS_TOKEN"
fi
req GET /classes/not-a-uuid 400 -n 'malformed class id rejected' "" -H "Authorization: Bearer $ACCESS_TOKEN"
req GET "/classes/11111111-1111-1111-1111-111111111111" 404 -n 'missing class not found' "" -H "Authorization: Bearer $ACCESS_TOKEN"

# 5. Patch the class and re-fetch.
if [[ -n "$CL_ID" ]]; then
    req PATCH "/classes/$CL_ID" 200 -n 'update class' '{"name":"Spin Advance","capacity":15}' \
        -H "Authorization: Bearer $ACCESS_TOKEN"
    req GET "/classes/$CL_ID" 200 -n 'fetch updated class' "" -H "Authorization: Bearer $ACCESS_TOKEN"
fi
req PATCH "/classes/11111111-1111-1111-1111-111111111111" 404 -n 'patch missing class returns 404' '{"name":"Nope"}' \
    -H "Authorization: Bearer $ACCESS_TOKEN"

# 6. Validation: ends-before-starts is 400, unknown branch is 404.
if [[ -n "$BR_ID" ]]; then
    NOW="$(python3 -c 'import datetime
print(datetime.datetime.now(datetime.timezone.utc).isoformat())')"
    req POST /classes 400 -n 'class ending before start rejected' "$(python3 -c 'import json,sys,datetime
print(json.dumps({"branchId": sys.argv[1], "name": "Bad Time",
                    "capacity": 5, "startsAt": sys.argv[2],
                    "endsAt": (datetime.datetime.fromisoformat(sys.argv[2])-datetime.timedelta(hours=1)).isoformat()}))' "$BR_ID" "$NOW")" \
        -H "Authorization: Bearer $ACCESS_TOKEN"
    req POST /classes 404 -n 'class on missing branch rejected' '{"branchId":"11111111-1111-1111-1111-111111111111","name":"Ghost","capacity":5,"startsAt":"2027-01-01T10:00:00Z","endsAt":"2027-01-01T11:00:00Z"}' \
        -H "Authorization: Bearer $ACCESS_TOKEN"
fi

# 7. Delete the class; a second delete is a 404.
if [[ -n "$CL_ID" ]]; then
    req DELETE "/classes/$CL_ID" 204 -n 'delete class' "" -H "Authorization: Bearer $ACCESS_TOKEN"
    req DELETE "/classes/$CL_ID" 404 -n 'deleting an absent class returns 404' "" -H "Authorization: Bearer $ACCESS_TOKEN"
fi

summary