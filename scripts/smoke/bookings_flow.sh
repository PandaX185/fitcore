#!/usr/bin/env bash
# bookings_flow.sh — exercises the bookings endpoints against the real API:
#   create, get, duplicate, class-full, list-by-class, cancel, rebook-after-cancel.
#
# Expectations (from api/openapi.yaml + booking service rules):
#   POST   /bookings                  201 / 409 (duplicate, full) / 404 (missing class/member)
#   GET    /bookings/{id}             200 / 400 / 404
#   POST   /bookings/{id}/cancel      200
#   GET    /classes/{id}/bookings     200
# Rebooking after cancel exercises migration 000005 (partial unique index).
set -uo pipefail
SELF_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib.sh
source "$SELF_DIR/lib.sh"

: "${SMOKE_EMAIL:=admin@fitcore.local}"
: "${SMOKE_PASSWORD:=Password1!}"

preflight

echo "== bookings flow =="

login "$SMOKE_EMAIL" "$SMOKE_PASSWORD"

# 1. Branch, two members, and a capacity-1 class one day out.
BR_NAME="SmokeBkBr-$(date +%s)"
req POST /branches 201 -n 'provision branch for booking' "$(python3 -c 'import json,sys
print(json.dumps({"name": sys.argv[1], "address": "1 Bk St",
                    "latitude": 31.5, "longitude": -8.0}))' "$BR_NAME")" \
    -H "Authorization: Bearer $ACCESS_TOKEN"
BR_ID=$(printf '%s' "$SMOKE_BODY" | json_get id)

M1=""
M2=""
if [[ -n "$BR_ID" ]]; then
    req POST /members 201 -n 'provision first booker' "$(python3 -c 'import json,sys
print(json.dumps({"branchId": sys.argv[1], "name": "Booker One",
                    "email": sys.argv[2]}))' "$BR_ID" "$(date +%s%N)@smoke.local")" \
        -H "Authorization: Bearer $ACCESS_TOKEN"
    M1=$(printf '%s' "$SMOKE_BODY" | json_get id)
    req POST /members 201 -n 'provision second booker' "$(python3 -c 'import json,sys
print(json.dumps({"branchId": sys.argv[1], "name": "Booker Two",
                    "email": sys.argv[2]}))' "$BR_ID" "$(date +%s%N)@smoke.local")" \
        -H "Authorization: Bearer $ACCESS_TOKEN"
    M2=$(printf '%s' "$SMOKE_BODY" | json_get id)
fi
CL_ID=""
if [[ -n "$BR_ID" ]]; then
    STARTS="$(python3 -c 'import datetime
print((datetime.datetime.now(datetime.timezone.utc)+datetime.timedelta(days=1)).isoformat())')"
    ENDS="$(python3 -c 'import datetime
print((datetime.datetime.now(datetime.timezone.utc)+datetime.timedelta(days=1, hours=1)).isoformat())')"
    req POST /classes 201 -n 'provision capacity-one class' "$(python3 -c 'import json,sys
print(json.dumps({"branchId": sys.argv[1], "name": "Bookable",
                    "capacity": 1, "startsAt": sys.argv[2],
                    "endsAt": sys.argv[3]}))' "$BR_ID" "$STARTS" "$ENDS")" \
        -H "Authorization: Bearer $ACCESS_TOKEN"
    CL_ID=$(printf '%s' "$SMOKE_BODY" | json_get id)
fi

# 2. First member books the only seat.
BK_ID=""
if [[ -n "$CL_ID" && -n "$M1" ]]; then
    req POST /bookings 201 -n 'book a seat in the class' "$(python3 -c 'import json,sys
print(json.dumps({"classId": sys.argv[1], "memberId": sys.argv[2]}))' "$CL_ID" "$M1")" \
        -H "Authorization: Bearer $ACCESS_TOKEN"
    BK_ID=$(printf '%s' "$SMOKE_BODY" | json_get id)

    # 3. Fetch the booking.
    req GET "/bookings/$BK_ID" 200 -n 'fetch booking by id' "" -H "Authorization: Bearer $ACCESS_TOKEN"

    # 4. The same member rebooking is a duplicate → 409.
    req POST /bookings 409 -n 'duplicate booking rejected' "$(python3 -c 'import json,sys
print(json.dumps({"classId": sys.argv[1], "memberId": sys.argv[2]}))' "$CL_ID" "$M1")" \
        -H "Authorization: Bearer $ACCESS_TOKEN"

    # 5. The class is at capacity → 409 for a second member.
    if [[ -n "$M2" ]]; then
        req POST /bookings 409 -n 'full class booking rejected' "$(python3 -c 'import json,sys
print(json.dumps({"classId": sys.argv[1], "memberId": sys.argv[2]}))' "$CL_ID" "$M2")" \
            -H "Authorization: Bearer $ACCESS_TOKEN"
    fi

    # 6. Roll call for the class.
    req GET "/classes/$CL_ID/bookings" 200 -n 'list bookings for a class' "" -H "Authorization: Bearer $ACCESS_TOKEN"

    # 7. Cancel frees the seat; rebooking succeeds (000005), rebooking while
    #    booked is refused again.
    req POST "/bookings/$BK_ID/cancel" 200 -n 'cancel booking' "" -H "Authorization: Bearer $ACCESS_TOKEN"
    req POST /bookings 201 -n 'rebook after cancel' "$(python3 -c 'import json,sys
print(json.dumps({"classId": sys.argv[1], "memberId": sys.argv[2]}))' "$CL_ID" "$M1")" \
        -H "Authorization: Bearer $ACCESS_TOKEN"
    BK2=$(printf '%s' "$SMOKE_BODY" | json_get id)
    if [[ -n "$BK2" ]]; then
        req POST /bookings 409 -n 'rebook while already booked rejected' "$(python3 -c 'import json,sys
print(json.dumps({"classId": sys.argv[1], "memberId": sys.argv[2]}))' "$CL_ID" "$M1")" \
            -H "Authorization: Bearer $ACCESS_TOKEN"
    fi
fi

# 8. A member id that is nil-uuid is invalid → 400; a well-formed but missing
#    member is not found → 404.
req GET "/bookings/11111111-1111-1111-1111-111111111111" 404 -n 'missing booking not found' "" -H "Authorization: Bearer $ACCESS_TOKEN"
req GET "/bookings/00000000-0000-0000-0000-000000000000" 400 -n 'nil booking uuid rejected' "" -H "Authorization: Bearer $ACCESS_TOKEN"
req POST /bookings 400 -n 'nil member id rejected' '{"classId":"11111111-1111-1111-1111-111111111111","memberId":"00000000-0000-0000-0000-000000000000"}' \
    -H "Authorization: Bearer $ACCESS_TOKEN"
if [[ -n "$CL_ID" ]]; then
    req POST /bookings 404 -n 'missing member booking rejected' "$(python3 -c 'import json,sys
print(json.dumps({"classId": sys.argv[1], "memberId": "11111111-1111-1111-1111-111111111111"}))' "$CL_ID")" \
        -H "Authorization: Bearer $ACCESS_TOKEN"
fi

summary