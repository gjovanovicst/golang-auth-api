-- Rollback: Drop api_key_usages table
-- Reverses: 20260305_create_api_key_usages.sql

DROP TABLE IF EXISTS api_key_usages;

DELETE FROM schema_migrations WHERE version = '20260305_create_api_key_usages';
