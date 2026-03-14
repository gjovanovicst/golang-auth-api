-- Rollback: 20260314_add_app_link_paths
-- Description: Remove configurable email action link path fields from the applications table.

ALTER TABLE applications
    DROP COLUMN IF EXISTS reset_password_path,
    DROP COLUMN IF EXISTS magic_link_path,
    DROP COLUMN IF EXISTS verify_email_path;
