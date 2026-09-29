#!/usr/bin/env bash
set -euo pipefail
: "${DATABASE_URL:?DATABASE_URL is required}"
BACKUP_DIR="${BACKUP_DIR:-./backups/postgres}"
mkdir -p "$BACKUP_DIR"
ts="$(date -u +%Y%m%dT%H%M%SZ)"
file="$BACKUP_DIR/web-studio-img-$ts.dump"
pg_dump "$DATABASE_URL" --format=custom --no-owner --no-privileges --file="$file"
sha256sum "$file" > "$file.sha256"
echo "$file"
