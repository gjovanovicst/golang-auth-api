-- Migration: add two_fa_previous_method and two_fa_previous_secret columns to users
-- These columns store the prior 2FA method and secret when switching to backup_email 2FA,
-- so that disabling backup_email 2FA can restore the original method automatically.

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS two_fa_previous_method VARCHAR(20) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS two_fa_previous_secret  TEXT        NOT NULL DEFAULT '';

INSERT INTO schema_migrations (version, name, applied_at, success)
VALUES ('20260310_add_two_fa_previous_method', 'Add two_fa_previous_method and two_fa_previous_secret to users', NOW(), true)
ON CONFLICT (version) DO NOTHING;
