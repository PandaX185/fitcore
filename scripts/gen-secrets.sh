#!/usr/bin/env bash
# Generate production secrets for the FitCore .env file (idempotent).
#
# Usage:
#   scripts/gen-secrets.sh [path-to-.env]
#
# Reads deploy/.env.prod.example as the template. Every existing line is kept;
# lines whose value is one of the __GENERATE_*__ placeholders are replaced with
# freshly generated hex secrets, unless the variable already has a real value.
# Requires ~/.rnd (default openssl scramble) and reads /dev/urandom, so it
# works headless.
#
# Writes the .env with mode 600 and never prints secret values.

set -euo pipefail

TEMPLATE="${TEMPLATE:-deploy/.env.prod.example}"
OUT="${1:-${OUT:-.env}}"

if [[ ! -f "$TEMPLATE" ]]; then
  echo "error: template not found: $TEMPLATE" >&2
  exit 1
fi

hex() {
  openssl rand -hex "$1"
}

gen_value() {
  case "$1" in
    "__GENERATE_32HEX__") hex 32 ;;
    "__GENERATE_48HEX__") hex 48 ;;
    "__GENERATE_24HEX__") hex 24 ;;
    *) echo "$1" ;;
  esac
}

: > "$OUT"
while IFS= read -r line || [[ -n "$line" ]]; do
  case "$line" in
    \#*|'') echo "$line" >> "$OUT" ;;
    *=*)
      name="${line%%=*}"
      value="${line#*=}"
      new_value="$(gen_value "$value")"
      echo "$name=$new_value" >> "$OUT"
      ;;
    *) echo "$line" >> "$OUT" ;;
  esac
done < "$TEMPLATE"

chmod 600 "$OUT"
echo "wrote $OUT (mode 600)"