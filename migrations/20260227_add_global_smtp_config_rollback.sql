-- Rollback: Remove global/system-level SMTP configuration support
-- Reverts: 20260227_add_global_smtp_config

-- ============================================================================
-- 1. Remove any global configs (app_id IS NULL) before making column NOT NULL
-- ============================================================================
DELETE FROM email_server_configs WHERE app_id IS NULL;

-- ============================================================================
-- 2. Drop new indexes
-- ============================================================================
DROP INDEX IF EXISTS idx_email_server_configs_global_default;
DROP INDEX IF EXISTS idx_email_server_configs_app_default;

-- ============================================================================
-- 3. Make app_id NOT NULL again
-- ============================================================================
ALTER TABLE email_server_configs ALTER COLUMN app_id SET NOT NULL;

-- ============================================================================
-- 4. Drop and recreate FK constraint
-- ============================================================================
ALTER TABLE email_server_configs DROP CONSTRAINT IF EXISTS email_server_configs_app_id_fkey;
ALTER TABLE email_server_configs
    ADD CONSTRAINT email_server_configs_app_id_fkey
    FOREIGN KEY (app_id) REFERENCES applications(id) ON DELETE CASCADE;

-- ============================================================================
-- 5. Recreate original partial unique index
-- ============================================================================
CREATE UNIQUE INDEX IF NOT EXISTS idx_email_server_configs_app_default
    ON email_server_configs(app_id) WHERE is_default = TRUE;

-- ============================================================================
-- Remove Migration Record
-- ============================================================================
DELETE FROM schema_migrations WHERE version = '20260227_add_global_smtp_config';
