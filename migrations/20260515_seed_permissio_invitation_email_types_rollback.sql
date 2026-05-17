-- Rollback: Remove Permissio invitation email type and default template
-- Date: 2026-05-15

-- Remove template first (foreign key dependency)
DELETE FROM email_templates WHERE email_type_id = (SELECT id FROM email_types WHERE code = 'permissio_invitation') AND app_id IS NULL;

-- Remove email type
DELETE FROM email_types WHERE code = 'permissio_invitation';

-- Remove migration record
DELETE FROM schema_migrations WHERE version = '20260515_seed_permissio_invitation_email_types';
