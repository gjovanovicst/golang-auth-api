-- Add magic_link_enabled column to admin_accounts
ALTER TABLE admin_accounts ADD COLUMN IF NOT EXISTS magic_link_enabled BOOLEAN NOT NULL DEFAULT FALSE;
