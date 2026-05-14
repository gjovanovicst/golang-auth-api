-- Rollback: Remove Centrora invitation email types and default templates
-- Date: 2026-05-14

-- Remove templates first (foreign key dependency)
DELETE FROM email_templates WHERE email_type_id = (SELECT id FROM email_types WHERE code = 'centrora_env_invitation')     AND app_id IS NULL;
DELETE FROM email_templates WHERE email_type_id = (SELECT id FROM email_types WHERE code = 'centrora_project_invitation') AND app_id IS NULL;
DELETE FROM email_templates WHERE email_type_id = (SELECT id FROM email_types WHERE code = 'centrora_invitation')         AND app_id IS NULL;

-- Remove email types
DELETE FROM email_types WHERE code IN ('centrora_invitation', 'centrora_project_invitation', 'centrora_env_invitation');

-- Remove migration record
DELETE FROM schema_migrations WHERE version = '20260514_seed_centrora_invitation_email_types';
