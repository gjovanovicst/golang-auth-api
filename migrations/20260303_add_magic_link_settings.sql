-- Migration: Add magic link login settings
-- Date: 2026-03-03
-- Description: Adds magic_link_enabled column to the applications table to support
--              per-app passwordless login via email magic links.

ALTER TABLE applications ADD COLUMN magic_link_enabled BOOLEAN NOT NULL DEFAULT FALSE;

-- Record Migration
INSERT INTO schema_migrations (version, name, applied_at, success)
VALUES ('20260303_add_magic_link_settings', 'Add magic link login settings', NOW(), true)
ON CONFLICT (version) DO NOTHING;
