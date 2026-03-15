-- Rollback: Remove scopes and expiry notification columns from api_keys
-- Reverses: 20260305_add_api_key_scopes_columns.sql

ALTER TABLE api_keys DROP COLUMN IF EXISTS scopes;
ALTER TABLE api_keys DROP COLUMN IF EXISTS notified_7_days_at;
ALTER TABLE api_keys DROP COLUMN IF EXISTS notified_1_day_at;

DELETE FROM schema_migrations WHERE version = '20260305_add_api_key_scopes_columns';
