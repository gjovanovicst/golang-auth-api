-- Rollback: Remove api_keys table
-- Date: 2026-02-20

DROP INDEX IF EXISTS idx_api_keys_active_lookup;
DROP INDEX IF EXISTS idx_api_keys_expires_at;
DROP INDEX IF EXISTS idx_api_keys_is_revoked;
DROP INDEX IF EXISTS idx_api_keys_app_id;
DROP INDEX IF EXISTS idx_api_keys_key_type;
DROP INDEX IF EXISTS idx_api_keys_key_hash;
DROP TABLE IF EXISTS api_keys;
