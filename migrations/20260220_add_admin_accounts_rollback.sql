-- Rollback: Add Admin Accounts Table
-- Description: Removes the admin_accounts table and its indexes.

-- 1. Drop the table (cascades indexes)
DROP TABLE IF EXISTS admin_accounts;

-- 2. Remove migration record
DELETE FROM schema_migrations WHERE version = '20260220_add_admin_accounts';
