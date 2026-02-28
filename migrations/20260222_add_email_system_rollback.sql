-- Rollback: Remove email system tables
-- Date: 2026-02-22

DROP TABLE IF EXISTS email_templates;
DROP TABLE IF EXISTS email_types;
DROP TABLE IF EXISTS email_server_configs;

-- Remove migration record
DELETE FROM schema_migrations WHERE version = '20260222_add_email_system';
