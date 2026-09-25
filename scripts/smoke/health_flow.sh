#!/usr/bin/env bash
# health_flow.sh — public endpoints that require no authentication.
# Run: SMOKE_BASE=http://localhost:8080 scripts/smoke/health_flow.sh
set -uo pipefail
SELF_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib.sh
source "$SELF_DIR/lib.sh"

preflight

req GET /healthz 200 -n 'liveness healthz is publicly reachable'
req GET /readyz 200 -n 'readiness readyz is publicly reachable'

summary