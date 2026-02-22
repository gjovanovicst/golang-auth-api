-- Rollback: Remove per-application 2FA settings
-- Date: 2026-02-22

ALTER TABLE applications DROP COLUMN IF EXISTS two_fa_issuer_name;
ALTER TABLE applications DROP COLUMN IF EXISTS two_fa_enabled;
ALTER TABLE applications DROP COLUMN IF EXISTS two_fa_required;

-- Remove migration record
DELETE FROM schema_migrations WHERE version = '20260222_add_app_twofa_settings';
