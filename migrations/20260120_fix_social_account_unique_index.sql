-- Migration: Fix Social Account Unique Index for Multi-Tenancy
-- Description: Update the unique index on social_accounts to include app_id,
--              allowing the same social account (provider + provider_user_id) 
--              to exist in different applications.

-- 1. Drop the old unique index
DROP INDEX IF EXISTS idx_provider_user_id;

-- 2. Create new unique index that includes app_id
CREATE UNIQUE INDEX idx_provider_user_id_app_id ON social_accounts(app_id, provider, provider_user_id);

-- 3. Record Migration
INSERT INTO schema_migrations (version, name, applied_at, success) 
VALUES ('20260120_fix_social_account_unique_index', 'Fix Social Account Unique Index for Multi-Tenancy', NOW(), true)
ON CONFLICT (version) DO NOTHING;
