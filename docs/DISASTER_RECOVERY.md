# Disaster Recovery

## Targets
- PostgreSQL is the source of truth.
- RPO: 24 hours for the scheduled database backup.
- RTO: 60 minutes for a tested restore on the target environment.
- Object storage exports/assets must be included in the storage provider's versioned/replicated backup policy.

## Backup
Run `scripts/backup-postgres.sh` with `DATABASE_URL` and `BACKUP_DIR`.
Keep backups outside the application host and encrypt them at rest.

## Restore verification
1. Restore PostgreSQL into an isolated database.
2. Run migrations.
3. Run `scripts/restore-verify-postgres.sh`.
4. Verify users, projects, iterations, generations, provenance hash chains and usage ledger counts.
5. Verify object-storage keys referenced by active assets/exports.
6. Record restore timestamp, backup ID and operator.

## Retention
Deleted projects/assets are recoverable for 30 days. After `purge_after`, a scheduled purge job may permanently remove database rows and corresponding objects. Purge must be auditable and must never rewrite provenance history of retained projects.

## Incident checklist
- Stop destructive jobs.
- Identify the latest known-good backup.
- Restore to an isolated environment first.
- Verify migrations and provenance.
- Verify object storage references.
- Switch traffic only after application smoke tests pass.
- Preserve incident and restore logs.
