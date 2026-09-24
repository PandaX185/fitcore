#!/usr/bin/env bash
# auth_flow.sh — exercises every endpoint of the auth module against the real
# API, covering the full session lifecycle and its revocation semantics:
#   login (public) -> refresh rotation -> single-use replay rejected ->
#   rotated-out access rejected -> logout -> post-logout everything rejected
#
# Expectations encode the contract verified against the live stack:
#   POST /auth/login   200 -> {accessToken, refreshToken, tokenType, expiresIn}
#   POST /auth/refresh 200, rotates both tokens (jti of prior access revoked)
#   POST /auth/refresh 401 on replay of an already-consumed refresh token
#   POST /auth/logout  401 for an access token revoked by rotation
#   POST /auth/logout  204 for a live session, revokes access jti + refresh
#   POST /auth/refresh 401 for the refresh token revoked by logout
#   POST /auth/logout  401 for the access token revoked by logout
#
# Run: SMOKE_BASE=http://localhost:8080 scripts/smoke/auth_flow.sh
set -uo pipefail
SELF_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib.sh
source "$SELF_DIR/lib.sh"

: "${SMOKE_EMAIL:=admin@fitcore.local}"
: "${SMOKE_PASSWORD:=Password1!}"

preflight

echo "== auth flow (${SMOKE_EMAIL}) =="

# 1. Public login issues an access token + single-use refresh token.
login "$SMOKE_EMAIL" "$SMOKE_PASSWORD"
ACCESS_1="$ACCESS_TOKEN"
REFRESH_1="$REFRESH_TOKEN"
_pass=$(( _pass + 1 ))
printf 'PASS  POST /auth/login → 200 (login issued tokens)\n'

# 2. Refresh consumes the refresh token and rotates the pair.
req POST /auth/refresh 200 "{\"refreshToken\":\"$REFRESH_1\"}"
ACCESS_2=$(printf '%s' "$SMOKE_BODY" | json_get accessToken)
REFRESH_2=$(printf '%s' "$SMOKE_BODY" | json_get refreshToken)

# 3. Single-use refresh token cannot be replayed after rotation.
req POST /auth/refresh 401 "{\"refreshToken\":\"$REFRESH_1\"}"

# 4. Access token issued before rotation has its jti revoked.
req POST /auth/logout 401 "" -H "Authorization: Bearer $ACCESS_1"

# 5. Logout of the current session succeeds and revokes everything.
req POST /auth/logout 204 "" -H "Authorization: Bearer $ACCESS_2"

# 6. Refresh token revoked by the logout.
req POST /auth/refresh 401 "{\"refreshToken\":\"$REFRESH_2\"}"

# 7. Access token revoked by the logout.
req POST /auth/logout 401 "" -H "Authorization: Bearer $ACCESS_2"

summary