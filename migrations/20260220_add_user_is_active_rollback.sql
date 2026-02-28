-- Rollback: Add is_active Column to Users Table
-- Description: Removes the is_active column from the users table.

-- 1. Drop the column
ALTER TABLE users DROP COLUMN IF EXISTS is_active;

-- 2. Remove migration record
DELETE FROM schema_migrations WHERE version = '20260220_add_user_is_active';
