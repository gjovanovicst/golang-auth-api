-- Migration: Add Admin Accounts Table
-- Description: Creates the admin_accounts table for Admin GUI authentication.
--              Admin accounts are system-level and separate from regular users.
--              They are not scoped to any application or tenant.

-- 1. Create admin_accounts table
CREATE TABLE IF NOT EXISTS admin_accounts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username VARCHAR(255) NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_login_at TIMESTAMPTZ
);

-- 2. Create unique index on username
CREATE UNIQUE INDEX IF NOT EXISTS idx_admin_accounts_username ON admin_accounts(username);

-- 3. Record Migration
INSERT INTO schema_migrations (version, name, applied_at, success)
VALUES ('20260220_add_admin_accounts', 'Add Admin Accounts Table', NOW(), true)
ON CONFLICT (version) DO NOTHING;
