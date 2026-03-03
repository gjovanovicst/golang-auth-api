-- Remove magic_link_enabled column from admin_accounts
ALTER TABLE admin_accounts DROP COLUMN IF EXISTS magic_link_enabled;
