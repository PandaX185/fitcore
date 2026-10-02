#!/usr/bin/env bash
# Monthly restore drill: prove the latest backup can actually be restored.
#
#   scripts/restore-drill.sh [dump-file]
#
# Defaults to the newest dump in ./backups. Spins up a disposable postgres
# container on a scratch network, restores the dump into it, verifies the
# schema came back with data, then tears the container down. Prod is never
# touched. Exits nonzero if any step fails.

set -euo pipefail

BACKUP_DIR="${BACKUP_DIR:-./backups}"
DUMP="${1:-$(ls -1t "$BACKUP_DIR"/fitcore-*.dump 2>/dev/null | head -1)}"

if [[ -z "$DUMP" || ! -f "$DUMP" ]]; then
  echo "error: no backup found in $BACKUP_DIR/ (and none passed as arg)" >&2
  exit 1
fi

cid="fitcore-restore-drill-$$-$(date +%s)"
cleanup() {
  docker rm -f "$cid" >/dev/null 2>&1 || true
}
trap cleanup EXIT

echo "==> restoring $DUMP"
docker run -d --name "$cid" \
  -e POSTGRES_PASSWORD=drill \
  -e POSTGRES_USER=postgres \
  postgres:16 >/dev/null

for i in $(seq 1 30); do
  if docker exec "$cid" pg_isready -U postgres >/dev/null 2>&1; then
    break
  fi
  sleep 1
done

docker exec "$cid" createdb -U postgres fitcore
docker cp "$DUMP" "$cid:/tmp/backup.dump"
docker exec "$cid" pg_restore -U postgres -d fitcore --no-owner /tmp/backup.dump

tables=$(docker exec "$cid" psql -U postgres -d fitcore -tAc \
  "SELECT count(*) FROM information_schema.tables WHERE table_schema='public'")
branches=$(docker exec "$cid" psql -U postgres -d fitcore -tAc "SELECT count(*) FROM branches")

if [[ "$tables" == "0" ]]; then
  echo "error: restored database has no tables (tables=$tables)" >&2
  exit 1
fi
if [[ -z "$branches" || "$branches" == "0" ]]; then
  echo "error: restored branches table looks empty" >&2
  exit 1
fi

echo "==> drill PASSED: $tables tables, $branches branches restored from $(basename "$DUMP")"