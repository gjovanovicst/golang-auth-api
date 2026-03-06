-- Migration: Add webhook_endpoints and webhook_deliveries tables
-- Date: 2026-03-05
-- Description: Implements the webhook system (feature #11).
--   - webhook_endpoints: one registered URL per (app_id, event_type) pair;
--     holds the HMAC-SHA256 signing secret (never returned after creation).
--   - webhook_deliveries: full per-attempt delivery history with status code,
--     response body snippet, latency, error message, and next-retry timestamp
--     for exponential-backoff retries.

BEGIN;

-- ─── webhook_endpoints ───────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS webhook_endpoints (
    id          UUID        NOT NULL DEFAULT gen_random_uuid(),
    app_id      UUID        NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    event_type  VARCHAR(64) NOT NULL,
    url         TEXT        NOT NULL,
    secret      TEXT        NOT NULL,   -- HMAC-SHA256 key; shown only once at creation
    is_active   BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at  TIMESTAMPTZ,            -- soft-delete support

    CONSTRAINT pk_webhook_endpoints PRIMARY KEY (id),

    -- Enforce the 8 supported event types at the DB level
    CONSTRAINT chk_webhook_event_type CHECK (event_type IN (
        'user.registered',
        'user.verified',
        'user.login',
        'user.password_changed',
        '2fa.enabled',
        '2fa.disabled',
        'social.linked',
        'social.unlinked'
    ))
);

-- One endpoint per (app, event) — composite unique (excludes soft-deleted rows)
CREATE UNIQUE INDEX IF NOT EXISTS idx_webhook_app_event
    ON webhook_endpoints (app_id, event_type)
    WHERE deleted_at IS NULL;

-- Support filtering / listing by app
CREATE INDEX IF NOT EXISTS idx_webhook_endpoints_app_id
    ON webhook_endpoints (app_id);

-- Support soft-delete scans
CREATE INDEX IF NOT EXISTS idx_webhook_endpoints_deleted_at
    ON webhook_endpoints (deleted_at);

-- ─── webhook_deliveries ──────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS webhook_deliveries (
    id             UUID        NOT NULL DEFAULT gen_random_uuid(),
    endpoint_id    UUID        NOT NULL REFERENCES webhook_endpoints(id) ON DELETE CASCADE,
    app_id         UUID        NOT NULL,
    event_type     VARCHAR(64) NOT NULL,
    payload        TEXT        NOT NULL,   -- full JSON payload sent
    attempt        INT         NOT NULL DEFAULT 1,
    status_code    INT,                    -- HTTP response code; 0 = no response received
    response_body  TEXT,                   -- first ~1 KB of response body
    latency_ms     BIGINT,                 -- round-trip time in milliseconds
    success        BOOLEAN     NOT NULL DEFAULT FALSE,  -- true = 2xx response
    error_message  TEXT,                   -- network / timeout error text
    next_retry_at  TIMESTAMPTZ,            -- NULL = no further retries scheduled
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT pk_webhook_deliveries PRIMARY KEY (id)
);

-- Primary lookup: all deliveries for a given endpoint (delivery history view)
CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_endpoint_id
    ON webhook_deliveries (endpoint_id);

-- Support per-app delivery queries
CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_app_id
    ON webhook_deliveries (app_id);

-- Support filtering by event type
CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_event_type
    ON webhook_deliveries (event_type);

-- Support filtering by success flag (e.g. show only failures)
CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_success
    ON webhook_deliveries (success);

-- Retry worker polls on this column to find pending retries
CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_next_retry_at
    ON webhook_deliveries (next_retry_at)
    WHERE next_retry_at IS NOT NULL;

-- ─── Schema migration record ─────────────────────────────────────────────────

INSERT INTO schema_migrations (version, name, applied_at, success)
VALUES (
    '20260305_add_webhooks',
    'Add webhook_endpoints and webhook_deliveries tables',
    NOW(),
    TRUE
)
ON CONFLICT (version) DO NOTHING;

COMMIT;
