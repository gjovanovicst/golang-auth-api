-- Rollback: Fix Social Account Unique Index for Multi-Tenancy
-- WARNING: This will revert to global unique constraint on (provider, provider_user_id)

-- 1. Drop the new unique index
DROP INDEX IF EXISTS idx_provider_user_id_app_id;

-- 2. Recreate the old unique index
CREATE UNIQUE INDEX idx_provider_user_id ON social_accounts(provider, provider_user_id);

-- 3. Remove migration record
DELETE FROM schema_migrations WHERE version = '20260120_fix_social_account_unique_index';
