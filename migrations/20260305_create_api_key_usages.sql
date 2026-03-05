-- Migration: Create api_key_usages table for per-key daily usage analytics
-- Date: 2026-03-05
-- Description: Creates the api_key_usages table with a composite unique index on
--              (api_key_id, period_date) to store daily request counters per API key.
--              Rows are upserted from middleware via fire-and-forget increments.

CREATE TABLE IF NOT EXISTS api_key_usages (
    id            BIGSERIAL PRIMARY KEY,
    api_key_id    UUID        NOT NULL REFERENCES api_keys(id) ON DELETE CASCADE,
    period_date   DATE        NOT NULL,
    request_count BIGINT      NOT NULL DEFAULT 0,
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Composite unique index enables the ON CONFLICT DO UPDATE upsert in middleware
CREATE UNIQUE INDEX IF NOT EXISTS idx_api_key_usage_key_period
    ON api_key_usages (api_key_id, period_date);

-- Index for querying usage by key (range scans over period_date)
CREATE INDEX IF NOT EXISTS idx_api_key_usages_api_key_id
    ON api_key_usages (api_key_id);

-- Register this migration
INSERT INTO schema_migrations (version, name, applied_at, success)
VALUES ('20260305_create_api_key_usages', 'Create api_key_usages table for daily usage analytics', NOW(), true)
ON CONFLICT (version) DO NOTHING;
