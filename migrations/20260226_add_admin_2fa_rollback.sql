-- Rollback: Remove 2FA support from admin_accounts
-- Date: 2026-02-26

ALTER TABLE admin_accounts DROP COLUMN IF EXISTS two_fa_recovery_codes;
ALTER TABLE admin_accounts DROP COLUMN IF EXISTS two_fa_secret;
ALTER TABLE admin_accounts DROP COLUMN IF EXISTS two_fa_method;
ALTER TABLE admin_accounts DROP COLUMN IF EXISTS two_fa_enabled;
ALTER TABLE admin_accounts DROP COLUMN IF EXISTS email;

-- Remove migration record
DELETE FROM schema_migrations WHERE version = '20260226_add_admin_2fa';
