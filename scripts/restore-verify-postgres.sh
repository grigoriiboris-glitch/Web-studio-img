#!/usr/bin/env bash
set -euo pipefail
: "${DATABASE_URL:?DATABASE_URL is required}"
: "${BACKUP_FILE:?BACKUP_FILE is required}"
pg_restore --list "$BACKUP_FILE" >/dev/null
tmp_db="${VERIFY_DATABASE_URL:-$DATABASE_URL}"
psql "$tmp_db" -v ON_ERROR_STOP=1 -c "SELECT count(*) AS users FROM users; SELECT count(*) AS projects FROM projects; SELECT count(*) AS provenance_events FROM provenance_events; SELECT count(*) AS usage_entries FROM usage_ledger;"
echo "restore verification queries completed"
