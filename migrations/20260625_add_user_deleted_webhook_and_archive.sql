-- Migration: Add user.deleted webhook event type and archived_users table
-- Date: 2026-06-25
-- Description: Adds 'user.deleted' to the allowed webhook event types and creates
--   the archived_users table for GDPR-compliant user archival on deletion.
--   - Drop + recreate CHECK constraint on webhook_endpoints.event_type to include 'user.deleted'
--   - archived_users stores a pseudonymised or full profile record when a user is deleted
--   - archive_mode 'purge' = GDPR erasure (PII anonymized), 'archive' = preserve for audit

BEGIN;

-- ─── Update webhook event type constraint ─────────────────────────────────────

ALTER TABLE webhook_endpoints DROP CONSTRAINT IF EXISTS chk_webhook_event_type;

ALTER TABLE webhook_endpoints ADD CONSTRAINT chk_webhook_event_type CHECK (event_type IN (
    'user.registered',
    'user.verified',
    'user.login',
    'user.password_changed',
    'user.deleted',
    '2fa.enabled',
    '2fa.disabled',
    'social.linked',
    'social.unlinked'
));

-- ─── archived_users ────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS archived_users (
    id               UUID         NOT NULL DEFAULT gen_random_uuid(),
    user_id          UUID         NOT NULL,                    -- original user UUID (not an FK — user row is deleted)
    app_id           UUID         NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    email            VARCHAR(255) NOT NULL,                    -- anonymized or original per archive_mode
    name             VARCHAR(255) NOT NULL DEFAULT '',
    first_name       VARCHAR(100) NOT NULL DEFAULT '',
    last_name        VARCHAR(100) NOT NULL DEFAULT '',
    locale           VARCHAR(10)  NOT NULL DEFAULT '',
    email_verified   BOOLEAN      NOT NULL DEFAULT FALSE,
    is_active        BOOLEAN      NOT NULL DEFAULT TRUE,
    two_fa_enabled   BOOLEAN      NOT NULL DEFAULT FALSE,
    has_password     BOOLEAN      NOT NULL DEFAULT TRUE,
    social_providers TEXT         NOT NULL DEFAULT '',         -- comma-separated, e.g. "google,github"
    registered_at    TIMESTAMPTZ  NOT NULL,                    -- original users.created_at
    deleted_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),      -- when admin performed the deletion
    deleted_by       VARCHAR(255) NOT NULL DEFAULT '',         -- admin username or email
    archive_mode     VARCHAR(20)  NOT NULL DEFAULT 'purge',    -- 'purge' (GDPR erase) or 'archive' (preserve)
    metadata         JSONB        NOT NULL DEFAULT '{}',       -- extensible: reason, original values before anonymization
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    CONSTRAINT pk_archived_users PRIMARY KEY (id)
);

-- Look up archive records by original user ID
CREATE INDEX IF NOT EXISTS idx_archived_users_user_id
    ON archived_users (user_id);

-- Look up archive records by email (for audit queries)
CREATE INDEX IF NOT EXISTS idx_archived_users_email
    ON archived_users (email);

-- Support time-range queries for retention/purging
CREATE INDEX IF NOT EXISTS idx_archived_users_deleted_at
    ON archived_users (deleted_at);

-- ─── Schema migration record ─────────────────────────────────────────────────

INSERT INTO schema_migrations (version, name, applied_at, success)
VALUES (
    '20260625_add_user_deleted_webhook_and_archive',
    'Add user.deleted webhook event type and archived_users table',
    NOW(),
    TRUE
)
ON CONFLICT (version) DO NOTHING;

COMMIT;
