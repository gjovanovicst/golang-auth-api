-- Migration: 20260314_add_app_link_paths
-- Description: Add configurable email action link path fields to the applications table.
--              These allow each application to define custom path suffixes for the URLs
--              sent in transactional emails (password reset, magic link, email verification).
--              When empty, the system falls back to the hardcoded defaults:
--                reset_password_path → /reset-password
--                magic_link_path     → /magic-link
--                verify_email_path   → /verify-email

ALTER TABLE applications
    ADD COLUMN IF NOT EXISTS reset_password_path VARCHAR(500) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS magic_link_path     VARCHAR(500) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS verify_email_path   VARCHAR(500) NOT NULL DEFAULT '';
