-- Rollback: Remove per-application brute-force protection settings
-- Date: 2026-03-05

ALTER TABLE applications DROP COLUMN IF EXISTS bf_lockout_enabled;
ALTER TABLE applications DROP COLUMN IF EXISTS bf_lockout_threshold;
ALTER TABLE applications DROP COLUMN IF EXISTS bf_lockout_durations;
ALTER TABLE applications DROP COLUMN IF EXISTS bf_lockout_window;
ALTER TABLE applications DROP COLUMN IF EXISTS bf_lockout_tier_ttl;
ALTER TABLE applications DROP COLUMN IF EXISTS bf_delay_enabled;
ALTER TABLE applications DROP COLUMN IF EXISTS bf_delay_start_after;
ALTER TABLE applications DROP COLUMN IF EXISTS bf_delay_max_seconds;
ALTER TABLE applications DROP COLUMN IF EXISTS bf_delay_tier_ttl;
ALTER TABLE applications DROP COLUMN IF EXISTS bf_captcha_enabled;
ALTER TABLE applications DROP COLUMN IF EXISTS bf_captcha_site_key;
ALTER TABLE applications DROP COLUMN IF EXISTS bf_captcha_secret_key;
ALTER TABLE applications DROP COLUMN IF EXISTS bf_captcha_threshold;

-- Remove migration record
DELETE FROM schema_migrations WHERE version = '20260305_add_app_bruteforce_settings';
