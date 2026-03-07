-- Rollback: Remove OIDC Provider support
-- Date: 2026-03-06
-- Reverses: 20260306_add_oidc.sql
--
-- WARNING: This will permanently delete all OIDC clients and authorization
-- codes, and remove the OIDC columns from the applications table.
-- Back up the database before running this rollback in production.

BEGIN;

-- ─── Drop indexes on oidc_auth_codes ─────────────────────────────────────────

DROP INDEX IF EXISTS idx_oidc_auth_codes_client_used;
DROP INDEX IF EXISTS idx_oidc_auth_codes_expires_at;
DROP INDEX IF EXISTS idx_oidc_auth_codes_client_id;
DROP INDEX IF EXISTS idx_oidc_auth_codes_app_id;

-- ─── Drop oidc_auth_codes ────────────────────────────────────────────────────

DROP TABLE IF EXISTS oidc_auth_codes;

-- ─── Drop indexes on oidc_clients ────────────────────────────────────────────

DROP INDEX IF EXISTS idx_oidc_clients_app_id_active;
DROP INDEX IF EXISTS idx_oidc_clients_app_id;

-- ─── Drop oidc_clients ───────────────────────────────────────────────────────

DROP TABLE IF EXISTS oidc_clients;

-- ─── Remove OIDC columns from applications ───────────────────────────────────

ALTER TABLE applications
    DROP COLUMN IF EXISTS oidc_issuer_url,
    DROP COLUMN IF EXISTS oidc_id_token_ttl,
    DROP COLUMN IF EXISTS oidc_rsa_private_key,
    DROP COLUMN IF EXISTS oidc_enabled;

-- ─── Remove schema migration record ─────────────────────────────────────────

DELETE FROM schema_migrations WHERE version = '20260306_add_oidc';

COMMIT;
