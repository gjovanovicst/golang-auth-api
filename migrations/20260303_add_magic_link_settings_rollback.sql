-- Rollback: Remove magic link login settings
-- Date: 2026-03-03

ALTER TABLE applications DROP COLUMN IF EXISTS magic_link_enabled;

-- Remove migration record
DELETE FROM schema_migrations WHERE version = '20260303_add_magic_link_settings';
