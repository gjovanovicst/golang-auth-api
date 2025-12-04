-- Rollback: Drop migration tracking table
-- WARNING: This will remove all migration history!

BEGIN;

DROP INDEX IF EXISTS idx_schema_migrations_version;
DROP INDEX IF EXISTS idx_schema_migrations_applied_at;
DROP TABLE IF EXISTS schema_migrations;

COMMIT;

