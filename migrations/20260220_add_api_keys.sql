-- Migration: Add api_keys table
-- Date: 2026-02-20
-- Description: Creates the api_keys table for managing admin and per-application API keys.

CREATE TABLE IF NOT EXISTS api_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    key_type VARCHAR(10) NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT DEFAULT '',
    key_hash VARCHAR(64) NOT NULL,
    key_prefix VARCHAR(16) NOT NULL,
    key_suffix VARCHAR(4) NOT NULL,
    app_id UUID REFERENCES applications(id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ,
    last_used_at TIMESTAMPTZ,
    is_revoked BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes
CREATE UNIQUE INDEX IF NOT EXISTS idx_api_keys_key_hash ON api_keys(key_hash);
CREATE INDEX IF NOT EXISTS idx_api_keys_key_type ON api_keys(key_type);
CREATE INDEX IF NOT EXISTS idx_api_keys_app_id ON api_keys(app_id);
CREATE INDEX IF NOT EXISTS idx_api_keys_is_revoked ON api_keys(is_revoked);
CREATE INDEX IF NOT EXISTS idx_api_keys_expires_at ON api_keys(expires_at);

-- Composite index for middleware lookup: active, non-revoked keys by hash
CREATE INDEX IF NOT EXISTS idx_api_keys_active_lookup ON api_keys(key_hash, is_revoked) WHERE is_revoked = false;
