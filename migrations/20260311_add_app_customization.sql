-- Migration: 20260311_add_app_customization
-- Description: Add login page branding, password policy, and token TTL override columns
--              to the applications table. Add password_history (JSONB) and
--              password_changed_at to the users table.

-- applications: Login Page Branding
ALTER TABLE applications
    ADD COLUMN IF NOT EXISTS login_logo_url        VARCHAR(500) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS login_primary_color   VARCHAR(20)  NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS login_secondary_color VARCHAR(20)  NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS login_display_name    VARCHAR(200) NOT NULL DEFAULT '';

-- applications: Password Policy
ALTER TABLE applications
    ADD COLUMN IF NOT EXISTS pw_min_length     INTEGER NOT NULL DEFAULT 8,
    ADD COLUMN IF NOT EXISTS pw_max_length     INTEGER NOT NULL DEFAULT 128,
    ADD COLUMN IF NOT EXISTS pw_require_upper  BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS pw_require_lower  BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS pw_require_digit  BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS pw_require_symbol BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS pw_history_count  INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS pw_max_age_days   INTEGER NOT NULL DEFAULT 0;

-- applications: Token TTL overrides (0 = fall back to global env var defaults)
ALTER TABLE applications
    ADD COLUMN IF NOT EXISTS access_token_ttl_minutes INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS refresh_token_ttl_hours  INTEGER NOT NULL DEFAULT 0;

-- users: Password history (array of bcrypt hashes stored as JSONB) and change timestamp
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS password_history    JSONB,
    ADD COLUMN IF NOT EXISTS password_changed_at TIMESTAMPTZ;
