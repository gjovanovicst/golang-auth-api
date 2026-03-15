-- Migration: Add per-application brute-force protection settings
-- Date: 2026-03-05
-- Description: Adds nullable brute-force configuration columns to the applications table.
--              NULL values mean "use global default from environment variables".
--              Non-NULL values override the global defaults for that specific application.

-- Account Lockout settings
ALTER TABLE applications ADD COLUMN bf_lockout_enabled BOOLEAN DEFAULT NULL;
ALTER TABLE applications ADD COLUMN bf_lockout_threshold INTEGER DEFAULT NULL;
ALTER TABLE applications ADD COLUMN bf_lockout_durations VARCHAR(255) DEFAULT NULL;
ALTER TABLE applications ADD COLUMN bf_lockout_window VARCHAR(50) DEFAULT NULL;
ALTER TABLE applications ADD COLUMN bf_lockout_tier_ttl VARCHAR(50) DEFAULT NULL;

-- Progressive Delay settings
ALTER TABLE applications ADD COLUMN bf_delay_enabled BOOLEAN DEFAULT NULL;
ALTER TABLE applications ADD COLUMN bf_delay_start_after INTEGER DEFAULT NULL;
ALTER TABLE applications ADD COLUMN bf_delay_max_seconds INTEGER DEFAULT NULL;
ALTER TABLE applications ADD COLUMN bf_delay_tier_ttl VARCHAR(50) DEFAULT NULL;

-- CAPTCHA settings
ALTER TABLE applications ADD COLUMN bf_captcha_enabled BOOLEAN DEFAULT NULL;
ALTER TABLE applications ADD COLUMN bf_captcha_site_key VARCHAR(500) DEFAULT NULL;
ALTER TABLE applications ADD COLUMN bf_captcha_secret_key VARCHAR(500) DEFAULT NULL;
ALTER TABLE applications ADD COLUMN bf_captcha_threshold INTEGER DEFAULT NULL;

-- Record Migration
INSERT INTO schema_migrations (version, name, applied_at, success)
VALUES ('20260305_add_app_bruteforce_settings', 'Add per-application brute-force protection settings', NOW(), true)
ON CONFLICT (version) DO NOTHING;
