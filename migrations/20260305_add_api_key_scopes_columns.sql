-- Migration: Add scopes and expiry notification tracking columns to api_keys
-- Date: 2026-03-05
-- Description: Adds 'scopes' (granular permission strings), 'notified_7_days_at',
--              and 'notified_1_day_at' columns to support API key scoping and
--              expiry notification deduplication.

-- 1. Add scopes column (comma-separated resource:action strings, e.g. "users:read,auth:*")
ALTER TABLE api_keys
    ADD COLUMN IF NOT EXISTS scopes TEXT NOT NULL DEFAULT '';

-- 2. Add notification tracking columns for expiry warnings
ALTER TABLE api_keys
    ADD COLUMN IF NOT EXISTS notified_7_days_at TIMESTAMPTZ;

ALTER TABLE api_keys
    ADD COLUMN IF NOT EXISTS notified_1_day_at TIMESTAMPTZ;

-- Register this migration
INSERT INTO schema_migrations (version, name, applied_at, success)
VALUES ('20260305_add_api_key_scopes_columns', 'Add scopes and expiry notification columns to api_keys', NOW(), true)
ON CONFLICT (version) DO NOTHING;
