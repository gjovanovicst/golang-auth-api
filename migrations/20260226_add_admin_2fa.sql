-- Migration: Add 2FA support to admin_accounts
-- Date: 2026-02-26
-- Description: Adds email, 2FA fields to admin_accounts for optional two-factor authentication

-- Add email column (nullable, unique when present)
-- Use GORM's naming convention for the unique index so AutoMigrate won't conflict
ALTER TABLE admin_accounts ADD COLUMN IF NOT EXISTS email VARCHAR(255);
CREATE UNIQUE INDEX IF NOT EXISTS idx_admin_accounts_email ON admin_accounts(email);

-- Add 2FA columns
ALTER TABLE admin_accounts ADD COLUMN IF NOT EXISTS two_fa_enabled BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE admin_accounts ADD COLUMN IF NOT EXISTS two_fa_method VARCHAR(20) DEFAULT '';
ALTER TABLE admin_accounts ADD COLUMN IF NOT EXISTS two_fa_secret TEXT DEFAULT '';
ALTER TABLE admin_accounts ADD COLUMN IF NOT EXISTS two_fa_recovery_codes JSONB DEFAULT '[]'::jsonb;

-- Record migration
INSERT INTO schema_migrations (version, name, applied_at, success)
VALUES ('20260226_add_admin_2fa', 'add_admin_2fa', NOW(), true)
ON CONFLICT (version) DO NOTHING;
