-- Migration: Add OIDC Provider support
-- Date: 2026-03-06
-- Description: Implements the OIDC Provider feature.
--   - Extends `applications` with per-app OIDC settings and RSA key storage.
--   - Creates `oidc_clients` table for registered relying-party clients.
--   - Creates `oidc_auth_codes` table for single-use authorization codes.
--
-- This migration is IDEMPOTENT (uses IF NOT EXISTS / IF NOT EXISTS guards)
-- so it is safe to run alongside GORM AutoMigrate which may have already
-- created the tables on first boot.

BEGIN;

-- ─── applications: add OIDC columns ──────────────────────────────────────────

ALTER TABLE applications
    ADD COLUMN IF NOT EXISTS oidc_enabled         BOOLEAN      NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS oidc_rsa_private_key TEXT         NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS oidc_id_token_ttl    INTEGER      NOT NULL DEFAULT 3600,
    ADD COLUMN IF NOT EXISTS oidc_issuer_url      VARCHAR(500) NOT NULL DEFAULT '';

COMMENT ON COLUMN applications.oidc_enabled         IS 'Master switch: expose OIDC endpoints for this application';
COMMENT ON COLUMN applications.oidc_rsa_private_key IS 'PKCS#8 PEM-encoded RSA-2048 private key used to sign ID tokens (RS256). Generated on first use. Never returned via API.';
COMMENT ON COLUMN applications.oidc_id_token_ttl    IS 'ID token lifetime in seconds (default 3600 = 1 hour)';
COMMENT ON COLUMN applications.oidc_issuer_url      IS 'Optional custom issuer URL override. Empty = auto-generated from PUBLIC_URL + app_id.';

-- ─── oidc_clients ─────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS oidc_clients (
    id                  UUID         NOT NULL DEFAULT gen_random_uuid(),
    app_id              UUID         NOT NULL REFERENCES applications(id) ON DELETE CASCADE,

    -- Human-readable name shown on the consent screen
    name                VARCHAR(100) NOT NULL,
    description         TEXT         NOT NULL DEFAULT '',

    -- OIDC client credentials
    -- client_id is the public identifier; client_secret_hash is bcrypt-hashed
    client_id           VARCHAR(64)  NOT NULL,
    client_secret_hash  TEXT         NOT NULL,

    -- JSON array of allowed redirect URIs, e.g. '["https://app.example.com/cb"]'
    redirect_uris       TEXT         NOT NULL DEFAULT '[]',

    -- Comma-separated allowed grant types: authorization_code, client_credentials, refresh_token
    allowed_grant_types VARCHAR(200) NOT NULL DEFAULT 'authorization_code,refresh_token',

    -- Comma-separated allowed OIDC scopes: openid, profile, email, roles
    allowed_scopes      VARCHAR(200) NOT NULL DEFAULT 'openid profile email',

    -- Behaviour flags
    require_consent     BOOLEAN      NOT NULL DEFAULT TRUE,
    is_confidential     BOOLEAN      NOT NULL DEFAULT TRUE,
    pkce_required       BOOLEAN      NOT NULL DEFAULT FALSE,
    is_active           BOOLEAN      NOT NULL DEFAULT TRUE,

    -- Optional logo shown on the consent screen
    logo_url            TEXT         NOT NULL DEFAULT '',

    created_at          TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    CONSTRAINT pk_oidc_clients PRIMARY KEY (id),
    CONSTRAINT uq_oidc_clients_client_id UNIQUE (client_id)
);

COMMENT ON TABLE  oidc_clients                  IS 'Registered OIDC/OAuth2 relying-party clients, scoped per application.';
COMMENT ON COLUMN oidc_clients.client_id        IS 'Public client identifier — safe to expose to end users.';
COMMENT ON COLUMN oidc_clients.client_secret_hash IS 'bcrypt hash of the client secret. The plaintext is shown only once on creation/rotation.';
COMMENT ON COLUMN oidc_clients.redirect_uris    IS 'JSON array of registered redirect URIs.';
COMMENT ON COLUMN oidc_clients.require_consent  IS 'When true the consent screen is shown; when false all scopes are auto-approved.';
COMMENT ON COLUMN oidc_clients.is_confidential  IS 'Confidential clients authenticate with a secret; public clients use PKCE only.';
COMMENT ON COLUMN oidc_clients.pkce_required    IS 'Enforce PKCE code_challenge even for confidential clients.';

-- Index: filter / list clients by application
CREATE INDEX IF NOT EXISTS idx_oidc_clients_app_id
    ON oidc_clients (app_id);

-- Index: quickly find active clients during authorization
CREATE INDEX IF NOT EXISTS idx_oidc_clients_app_id_active
    ON oidc_clients (app_id, is_active);

-- ─── oidc_auth_codes ──────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS oidc_auth_codes (
    id                    UUID         NOT NULL DEFAULT gen_random_uuid(),
    app_id                UUID         NOT NULL,
    client_id             VARCHAR(64)  NOT NULL,
    user_id               UUID         NOT NULL,

    -- The random single-use code sent to the redirect_uri
    code                  VARCHAR(128) NOT NULL,

    -- redirect_uri must exactly match the value used in the token request
    redirect_uri          TEXT         NOT NULL,

    -- Space-separated granted scopes
    scopes                TEXT         NOT NULL,

    -- Nonce echoed into the ID token (empty string if not provided)
    nonce                 TEXT         NOT NULL DEFAULT '',

    -- PKCE
    code_challenge        TEXT         NOT NULL DEFAULT '',
    code_challenge_method VARCHAR(10)  NOT NULL DEFAULT '',   -- "S256" or ""

    -- Expiry and replay protection
    expires_at            TIMESTAMPTZ  NOT NULL,
    used                  BOOLEAN      NOT NULL DEFAULT FALSE,

    created_at            TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    CONSTRAINT pk_oidc_auth_codes PRIMARY KEY (id),
    CONSTRAINT uq_oidc_auth_codes_code UNIQUE (code)
);

COMMENT ON TABLE  oidc_auth_codes            IS 'Single-use authorization codes issued during the OAuth2 Authorization Code flow.';
COMMENT ON COLUMN oidc_auth_codes.code       IS 'Random single-use code sent to the redirect_uri. Exchanged at /token within the expiry window.';
COMMENT ON COLUMN oidc_auth_codes.used       IS 'Set to true after the code is exchanged to prevent replay attacks.';
COMMENT ON COLUMN oidc_auth_codes.expires_at IS 'Codes expire after a short window (configurable, default 10 minutes).';

-- Index: filter by app
CREATE INDEX IF NOT EXISTS idx_oidc_auth_codes_app_id
    ON oidc_auth_codes (app_id);

-- Index: lookup by client during token exchange
CREATE INDEX IF NOT EXISTS idx_oidc_auth_codes_client_id
    ON oidc_auth_codes (client_id);

-- Index: expiry cleanup worker and validity checks
CREATE INDEX IF NOT EXISTS idx_oidc_auth_codes_expires_at
    ON oidc_auth_codes (expires_at);

-- Index: token exchange query — find unused codes by client
CREATE INDEX IF NOT EXISTS idx_oidc_auth_codes_client_used
    ON oidc_auth_codes (client_id, used);

-- ─── Schema migration record ─────────────────────────────────────────────────

INSERT INTO schema_migrations (version, name, applied_at, success)
VALUES (
    '20260306_add_oidc',
    'Add OIDC Provider support (oidc_clients, oidc_auth_codes, applications OIDC columns)',
    NOW(),
    TRUE
)
ON CONFLICT (version) DO NOTHING;

COMMIT;
