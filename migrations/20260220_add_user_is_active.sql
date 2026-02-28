-- Migration: Add is_active Column to Users Table
-- Description: Adds an is_active boolean column to the users table.
--              Defaults to TRUE so all existing users remain active.
--              Used by the Admin GUI to deactivate/reactivate user accounts.

-- 1. Add is_active column with default true
ALTER TABLE users ADD COLUMN IF NOT EXISTS is_active BOOLEAN NOT NULL DEFAULT TRUE;

-- 2. Record Migration
INSERT INTO schema_migrations (version, name, applied_at, success)
VALUES ('20260220_add_user_is_active', 'Add is_active Column to Users Table', NOW(), true)
ON CONFLICT (version) DO NOTHING;
