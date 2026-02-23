-- Rollback: Remove email 2FA settings from applications and users
-- Date: 2026-02-22

ALTER TABLE users DROP COLUMN IF EXISTS two_fa_method;
ALTER TABLE applications DROP COLUMN IF EXISTS two_fa_methods;
ALTER TABLE applications DROP COLUMN IF EXISTS email_2fa_enabled;

-- Remove migration record
DELETE FROM schema_migrations WHERE version = '20260222_add_email_2fa_settings';
