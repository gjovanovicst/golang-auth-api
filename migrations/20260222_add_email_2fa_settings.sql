-- Migration: Add email 2FA settings to applications and users
-- Date: 2026-02-22
-- Description: Adds columns to support email-based 2FA:
--   - applications.email_2fa_enabled: Allow email-based 2FA for the application
--   - applications.two_fa_methods: Comma-separated list of available 2FA methods
--   - users.two_fa_method: User's chosen 2FA method (totp or email)

ALTER TABLE applications ADD COLUMN IF NOT EXISTS email_2fa_enabled BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE applications ADD COLUMN IF NOT EXISTS two_fa_methods VARCHAR(50) NOT NULL DEFAULT 'totp';

ALTER TABLE users ADD COLUMN IF NOT EXISTS two_fa_method VARCHAR(20) NOT NULL DEFAULT '';

-- Record Migration
INSERT INTO schema_migrations (version, name, applied_at, success)
VALUES ('20260222_add_email_2fa_settings', 'Add email 2FA settings to applications and users', NOW(), true)
ON CONFLICT (version) DO NOTHING;
