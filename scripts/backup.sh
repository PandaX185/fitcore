#!/usr/bin/env bash
# Nightly database backup for the FitCore production stack.
#
#   scripts/backup.sh [compose-file] [backup-dir] [dump-name]
#
# Requires POSTGRES_PASSWORD in the environment (the prod Justfile source .env).
# Produces one compressed custom-format dump per run and keeps the 14 most
# recent dumps. Restore is exercised monthly by scripts/restore-drill.sh.
#
# RPO: daily by default; pair with WAL archiving (docs/PROD.md § backups) for a
# point-in-time recovery window.

set -euo pipefail

COMPOSE_FILE="${1:-deploy/docker-compose.prod.yml}"
BACKUP_DIR="${2:-./backups}"
NAME="${3:-fitcore-$(date -u +%Y%m%d-%H%M%S).dump}"

if [[ -z "${POSTGRES_PASSWORD:-}" ]]; then
  echo "error: POSTGRES_PASSWORD is required (source the prod .env first)" >&2
  exit 1
fi

mkdir -p "$BACKUP_DIR"
target="$BACKUP_DIR/$NAME"

# -Fc custom format: compressed by default and restorable selectively.
docker compose -f "$COMPOSE_FILE" exec -T \
  -e PGPASSWORD="$POSTGRES_PASSWORD" \
  postgres pg_dump -U fitcore -d fitcore -Fc -f /var/lib/postgresql/data/backup.dump

# Copy the dump out of the container volume into the host dir, then delete the
# in-container copy so it cannot grow unbounded inside the volume.
docker compose -f "$COMPOSE_FILE" cp postgres:/var/lib/postgresql/data/backup.dump "$target"
docker compose -f "$COMPOSE_FILE" exec -T postgres rm -f /var/lib/postgresql/data/backup.dump

# Retention: keep the 14 newest dumps.
ls -1t "$BACKUP_DIR"/fitcore-*.dump 2>/dev/null | tail -n +15 | xargs -r rm -f

size=$(du -h "$target" | cut -f1)
echo "backup written: $target ($size)"

# Optional off-site copy: BACKUP_RCLONE_REMOTE=<remote>:<dir> (rclone on host)
if [[ -n "${BACKUP_RCLONE_REMOTE:-}" ]]; then
  rclone copy "$target" "$BACKUP_RCLONE_REMOTE" --log-file=/dev/null
  echo "copied to rclone remote: $BACKUP_RCLONE_REMOTE"
fi