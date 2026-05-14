-- Step 0 Rollback: Remove global email uniqueness constraint
-- Reverts to per-app email uniqueness (composite index only).

DROP INDEX IF EXISTS idx_users_email_global;

-- Record Migration removal
DELETE FROM schema_migrations WHERE version = '20260508_add_global_email_unique';
