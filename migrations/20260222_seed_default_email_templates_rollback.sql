-- Rollback: Seed Default Email Templates
-- Date: 2026-02-22
-- Description: Remove the 6 seeded global default email templates.
--              The hardcoded defaults in defaults.go will continue to serve as fallback.

-- Remove all global default templates (app_id IS NULL)
-- Only deletes templates that have no app_id (global defaults seeded by this migration).
-- App-specific templates created by admins are NOT affected.
DELETE FROM email_templates WHERE app_id IS NULL;

-- Remove migration record
DELETE FROM schema_migrations WHERE version = '20260222_seed_default_email_templates';
