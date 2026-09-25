#!/usr/bin/env bash
# billing_flow.sh — exercises the invoices endpoints against the real API:
#   create, get, list-by-member, update (paid), and refusal cases.
#
# Expectations (from api/openapi.yaml + billing service rules):
#   POST  /invoices          201 / 400 (bad currency) / 404 (missing member/membership)
#   GET   /invoices/{id}     200 / 400 / 404
#   PATCH /invoices/{id}     200 / 400 (invalid status, paid->revert) / 404
#   GET   /members/{id}/invoices 200
set -uo pipefail
SELF_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib.sh
source "$SELF_DIR/lib.sh"

: "${SMOKE_EMAIL:=admin@fitcore.local}"
: "${SMOKE_PASSWORD:=Password1!}"

preflight

echo "== billing flow =="

login "$SMOKE_EMAIL" "$SMOKE_PASSWORD"

# 1. Branch + member + membership to bill against.
BR_NAME="SmokeInvBr-$(date +%s)"
req POST /branches 201 -n 'provision branch for billing' "$(python3 -c 'import json,sys
print(json.dumps({"name": sys.argv[1], "address": "1 Inv St",
                    "latitude": 31.5, "longitude": -8.0}))' "$BR_NAME")" \
    -H "Authorization: Bearer $ACCESS_TOKEN"
BR_ID=$(printf '%s' "$SMOKE_BODY" | json_get id)
MB_ID=""
if [[ -n "$BR_ID" ]]; then
    req POST /members 201 -n 'provision member for billing' "$(python3 -c 'import json,sys
print(json.dumps({"branchId": sys.argv[1], "name": "Invoicee",
                    "email": sys.argv[2]}))' "$BR_ID" "$(date +%s%N)@smoke.local")" \
        -H "Authorization: Bearer $ACCESS_TOKEN"
    MB_ID=$(printf '%s' "$SMOKE_BODY" | json_get id)
fi
req POST /packages 201 -n 'provision package for billing' "$(python3 -c 'import json,sys
print(json.dumps({"name": "Pkg " + sys.argv[1], "durationDays": 30,
                    "priceCents": 25000, "currency": "BHD"}))' "$(date +%s%N)")" \
    -H "Authorization: Bearer $ACCESS_TOKEN"
PK_ID=$(printf '%s' "$SMOKE_BODY" | json_get id)
MS_ID=""
if [[ -n "$MB_ID" && -n "$PK_ID" && -n "$BR_ID" ]]; then
    req POST /memberships 201 -n 'provision membership for billing' "$(python3 -c 'import json,sys
print(json.dumps({"memberId": sys.argv[1], "packageId": sys.argv[2],
                    "branchId": sys.argv[3]}))' "$MB_ID" "$PK_ID" "$BR_ID")" \
        -H "Authorization: Bearer $ACCESS_TOKEN"
    MS_ID=$(printf '%s' "$SMOKE_BODY" | json_get id)
fi

# 2. Issue an invoice, due in 7 days.
INV_ID=""
if [[ -n "$MB_ID" && -n "$MS_ID" ]]; then
    DUE="$(python3 -c 'import datetime
print((datetime.datetime.now(datetime.timezone.utc)+datetime.timedelta(days=7)).isoformat())')"
    req POST /invoices 201 -n 'issue an invoice' "$(python3 -c 'import json,sys
print(json.dumps({"memberId": sys.argv[1], "membershipId": sys.argv[2],
                    "amountCents": 25000, "currency": "bhd",
                    "dueAt": sys.argv[3]}))' "$MB_ID" "$MS_ID" "$DUE")" \
        -H "Authorization: Bearer $ACCESS_TOKEN"
    INV_ID=$(printf '%s' "$SMOKE_BODY" | json_get id)

    # 3. Fetch by ID and list by member.
    if [[ -n "$INV_ID" ]]; then
        req GET "/invoices/$INV_ID" 200 -n 'fetch invoice by id' "" -H "Authorization: Bearer $ACCESS_TOKEN"
    fi
    req GET "/members/$MB_ID/invoices" 200 -n 'list invoices by member' "" -H "Authorization: Bearer $ACCESS_TOKEN"

    # 4. Mark paid; reverting paid->pending is refused.
    if [[ -n "$INV_ID" ]]; then
        req PATCH "/invoices/$INV_ID" 200 -n 'mark invoice paid' '{"status":"paid"}' \
            -H "Authorization: Bearer $ACCESS_TOKEN"
        req PATCH "/invoices/$INV_ID" 400 -n 'reverting a paid invoice rejected' '{"status":"pending"}' \
            -H "Authorization: Bearer $ACCESS_TOKEN"
    fi
    req PATCH "/invoices/$INV_ID" 400 -n 'invalid invoice status rejected' '{"status":"garbage"}' \
        -H "Authorization: Bearer $ACCESS_TOKEN"

    # 5. Bad currency is refused at issue time.
    req POST /invoices 400 -n 'invalid invoice currency rejected' "$(python3 -c 'import json,sys
print(json.dumps({"memberId": sys.argv[1], "membershipId": sys.argv[2],
                    "amountCents": 100, "currency": "XX",
                    "dueAt": sys.argv[3]}))' "$MB_ID" "$MS_ID" "$DUE")" \
        -H "Authorization: Bearer $ACCESS_TOKEN"

    # 6. A well-formed but missing member is not found → 404.
    req POST /invoices 404 -n 'invoice for missing member rejected' "$(python3 -c 'import json,sys
print(json.dumps({"memberId": "11111111-1111-1111-1111-111111111111",
                    "membershipId": sys.argv[1],
                    "amountCents": 100, "currency": "USD",
                    "dueAt": sys.argv[2]}))' "$MS_ID" "$DUE")" \
        -H "Authorization: Bearer $ACCESS_TOKEN"
fi

# 7. Missing and malformed lookups; a nil membership id is invalid → 400.
req GET "/invoices/11111111-1111-1111-1111-111111111111" 404 -n 'missing invoice not found' "" -H "Authorization: Bearer $ACCESS_TOKEN"
req GET "/invoices/00000000-0000-0000-0000-000000000000" 400 -n 'nil invoice uuid rejected' "" -H "Authorization: Bearer $ACCESS_TOKEN"
req POST /invoices 400 -n 'nil membership id rejected' '{"memberId":"11111111-1111-1111-1111-111111111111","membershipId":"00000000-0000-0000-0000-000000000000","amountCents":100,"currency":"USD","dueAt":"2026-12-01T00:00:00Z"}' \
    -H "Authorization: Bearer $ACCESS_TOKEN"

summary