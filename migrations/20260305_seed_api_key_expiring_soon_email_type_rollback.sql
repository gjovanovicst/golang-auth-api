-- Rollback: Remove api_key_expiring_soon email type and its default template
-- Reverses: 20260305_seed_api_key_expiring_soon_email_type.sql

-- 1. Delete the global default template first (foreign key constraint)
DELETE FROM email_templates
WHERE email_type_id = (SELECT id FROM email_types WHERE code = 'api_key_expiring_soon')
  AND app_id IS NULL;

-- 2. Delete the email type
DELETE FROM email_types WHERE code = 'api_key_expiring_soon';

-- 3. Remove migration record
DELETE FROM schema_migrations WHERE version = '20260305_seed_api_key_expiring_soon_email_type';
