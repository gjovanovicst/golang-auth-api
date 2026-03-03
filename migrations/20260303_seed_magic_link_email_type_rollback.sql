-- Rollback: Remove magic link email type and default template
-- Date: 2026-03-03

-- Remove template first (foreign key dependency)
DELETE FROM email_templates WHERE email_type_id = (SELECT id FROM email_types WHERE code = 'magic_link') AND app_id IS NULL;

-- Remove the email type
DELETE FROM email_types WHERE code = 'magic_link';

-- Remove migration record
DELETE FROM schema_migrations WHERE version = '20260303_seed_magic_link_email_type';
