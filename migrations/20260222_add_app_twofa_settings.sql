-- Migration: Add per-application 2FA settings
-- Date: 2026-02-22
-- Description: Adds three columns to the applications table to support per-app 2FA configuration:
--   - two_fa_issuer_name: Custom display name for authenticator apps (overrides app name)
--   - two_fa_enabled: Master switch to enable/disable 2FA feature per application
--   - two_fa_required: Force all users of this application to set up 2FA

ALTER TABLE applications ADD COLUMN two_fa_issuer_name TEXT NOT NULL DEFAULT '';
ALTER TABLE applications ADD COLUMN two_fa_enabled BOOLEAN NOT NULL DEFAULT TRUE;
ALTER TABLE applications ADD COLUMN two_fa_required BOOLEAN NOT NULL DEFAULT FALSE;

-- Record Migration
INSERT INTO schema_migrations (version, name, applied_at, success)
VALUES ('20260222_add_app_twofa_settings', 'Add per-application 2FA settings', NOW(), true)
ON CONFLICT (version) DO NOTHING;
