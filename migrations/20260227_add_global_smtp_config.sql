-- Migration: Add global/system-level SMTP configuration support
-- Date: 2026-02-27
-- Description: Makes email_server_configs.app_id nullable so that a config with
--   NULL app_id serves as the system-level/global SMTP configuration.
--   Resolution chain: app-specific config -> global config -> dev mode (log to stdout).

-- ============================================================================
-- 1. Make app_id nullable
-- ============================================================================

-- Drop the existing partial unique index on (app_id WHERE is_default = TRUE)
-- This index was created in the multi-sender migration and requires app_id to be NOT NULL
DROP INDEX IF EXISTS idx_email_server_configs_app_default;

-- Drop the existing foreign key constraint on app_id
-- PostgreSQL auto-names FK constraints as <table>_<column>_fkey
ALTER TABLE email_server_configs DROP CONSTRAINT IF EXISTS email_server_configs_app_id_fkey;

-- Make app_id nullable
ALTER TABLE email_server_configs ALTER COLUMN app_id DROP NOT NULL;

-- ============================================================================
-- 2. Re-add FK constraint (now allows NULL, FK is only enforced for non-NULL values)
-- ============================================================================
ALTER TABLE email_server_configs
    ADD CONSTRAINT email_server_configs_app_id_fkey
    FOREIGN KEY (app_id) REFERENCES applications(id) ON DELETE CASCADE;

-- ============================================================================
-- 3. Recreate partial unique indexes for default configs
-- ============================================================================

-- Only one default per app (for app-scoped configs)
CREATE UNIQUE INDEX IF NOT EXISTS idx_email_server_configs_app_default
    ON email_server_configs(app_id) WHERE app_id IS NOT NULL AND is_default = TRUE;

-- Only one active+default global config (app_id IS NULL)
CREATE UNIQUE INDEX IF NOT EXISTS idx_email_server_configs_global_default
    ON email_server_configs(is_default) WHERE app_id IS NULL AND is_default = TRUE;

-- ============================================================================
-- Record Migration
-- ============================================================================
INSERT INTO schema_migrations (version, name, applied_at, success)
VALUES ('20260227_add_global_smtp_config', 'Add global/system-level SMTP configuration support', NOW(), true)
ON CONFLICT (version) DO NOTHING;
