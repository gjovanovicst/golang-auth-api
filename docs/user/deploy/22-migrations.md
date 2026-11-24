# Migrations

The schema has two layers, and understanding which one owns what prevents most
deployment surprises.

## 1. GORM auto-migration (runs on every boot)

On startup `MigrateDatabase()` calls `AutoMigrate` for every model. It is additive and
idempotent:

- creates missing tables, columns and indexes;
- never drops a column, never changes a column type;
- logs a warning and continues if a SQL migration conflicts with a model.

This is why a fresh database "just works" — and why anything destructive (renames, type
changes, `NOT NULL` on existing data) must be a SQL migration.

## 2. SQL migrations (`migrations/*.sql`)

Versioned files applied in filename order and tracked in `schema_migrations`:

```
migrations/
  20260105_add_multi_tenancy.sql
  20260105_add_multi_tenancy_rollback.sql
  20260301_seed_rbac_defaults.sql
  …
```

The `_rollback.sql` sibling reverses the change. Each forward file should be idempotent
(`IF NOT EXISTS`, `ON CONFLICT DO NOTHING`) because the runner does not wrap every file in
a transaction and may be re-run on a database whose auto-migration already added a column.

## Applying them

```bash
make migrate-up        # apply every pending .sql, skip rollback files
make migrate-status    # show tables (inside the Docker database)
make migrate-list      # list files
make migrate-down      # roll back the last applied migration
```

The Make targets talk to the database **inside the `auth_db` container**, which is the
Docker Compose setup. When you run PostgreSQL elsewhere, use the scripts directly with
your connection:

```bash
PGPASSWORD=… psql -h localhost -p 5432 -U postgres -d auth_db \
  -v ON_ERROR_STOP=1 -f migrations/20260105_add_multi_tenancy.sql
```

## Writing a migration

1. Copy `migrations/TEMPLATE.md` and create `<date>_<description>.sql` plus the
   `_rollback.sql` sibling.
2. Prefer `CREATE TABLE IF NOT EXISTS`, `ADD COLUMN IF NOT EXISTS`,
   `CREATE INDEX IF NOT EXISTS`.
3. Seed data with `ON CONFLICT DO NOTHING` so re-runs are safe.
4. Test on a scratch database: apply, roll back, apply again.
5. Record it in `migrations/MIGRATIONS_LOG.md` and, when behaviour changes, in
   `CHANGELOG.md`.

## The multi-tenancy baseline

`20260105_add_multi_tenancy.sql` creates `tenants`, `applications` and
`oauth_provider_configs`, and backfills `app_id` on existing rows. It assumes the legacy
single-tenant tables already exist (users, social_accounts, activity_logs). On a brand-new
database the auto-migration creates those tables first, so apply it **after** the first
boot.

`oauth_provider_configs` and `tenants` are now also covered by `AutoMigrate`, so a
database that was never touched by the SQL migration still gets the tables and indexes it
needs — but seeding the default tenant/application still comes from the SQL file.

## Checking the schema

```bash
# tables and row counts
psql -c "select table_name from information_schema.tables where table_schema='public' order by 1;"

# which migrations the runner recorded
psql -c "select version, applied_at, success from schema_migrations order by applied_at;"
```

## Rolling back a failed deployment

1. Stop the API (the old binary talks to the old schema).
2. Apply the failing migration's `_rollback.sql`.
3. Restore the previous image/binary.
4. Fix forward with a **new** migration — never edit an applied one.
