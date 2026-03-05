-- Rollback: Remove new_device_login and suspicious_activity email types and default templates
-- Date: 2026-03-05

-- Remove templates first (foreign key dependency)
DELETE FROM email_templates WHERE email_type_id = (SELECT id FROM email_types WHERE code = 'new_device_login') AND app_id IS NULL;
DELETE FROM email_templates WHERE email_type_id = (SELECT id FROM email_types WHERE code = 'suspicious_activity') AND app_id IS NULL;

-- Remove the email types
DELETE FROM email_types WHERE code = 'new_device_login';
DELETE FROM email_types WHERE code = 'suspicious_activity';

-- Remove migration record
DELETE FROM schema_migrations WHERE version = '20260305_seed_security_email_types';
