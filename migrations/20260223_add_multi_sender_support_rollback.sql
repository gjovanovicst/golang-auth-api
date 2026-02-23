-- Rollback: Remove multi-sender support
-- Date: 2026-02-23

-- Remove server_config_id from email_templates
ALTER TABLE email_templates DROP COLUMN IF EXISTS server_config_id;

-- Remove from_email and from_name from email_templates
ALTER TABLE email_templates DROP COLUMN IF EXISTS from_email;
ALTER TABLE email_templates DROP COLUMN IF EXISTS from_name;

-- Remove the partial unique index for default config
DROP INDEX IF EXISTS idx_email_server_configs_app_default;

-- Remove name and is_default from email_server_configs
ALTER TABLE email_server_configs DROP COLUMN IF EXISTS name;
ALTER TABLE email_server_configs DROP COLUMN IF EXISTS is_default;

-- Drop the non-unique index we created
DROP INDEX IF EXISTS idx_email_server_configs_app_id;

-- Restore the original unique constraint on app_id
ALTER TABLE email_server_configs ADD CONSTRAINT email_server_configs_app_id_key UNIQUE (app_id);

-- Remove migration record
DELETE FROM schema_migrations WHERE version = '20260223_add_multi_sender_support';
