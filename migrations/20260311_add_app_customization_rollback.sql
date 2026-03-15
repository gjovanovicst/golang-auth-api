-- Rollback: 20260311_add_app_customization
-- Removes login page branding, password policy, token TTL, and user password
-- history/expiry columns added in 20260311_add_app_customization.sql

-- applications: Token TTL overrides
ALTER TABLE applications
    DROP COLUMN IF EXISTS access_token_ttl_minutes,
    DROP COLUMN IF EXISTS refresh_token_ttl_hours;

-- applications: Password Policy
ALTER TABLE applications
    DROP COLUMN IF EXISTS pw_min_length,
    DROP COLUMN IF EXISTS pw_max_length,
    DROP COLUMN IF EXISTS pw_require_upper,
    DROP COLUMN IF EXISTS pw_require_lower,
    DROP COLUMN IF EXISTS pw_require_digit,
    DROP COLUMN IF EXISTS pw_require_symbol,
    DROP COLUMN IF EXISTS pw_history_count,
    DROP COLUMN IF EXISTS pw_max_age_days;

-- applications: Login Page Branding
ALTER TABLE applications
    DROP COLUMN IF EXISTS login_logo_url,
    DROP COLUMN IF EXISTS login_primary_color,
    DROP COLUMN IF EXISTS login_secondary_color,
    DROP COLUMN IF EXISTS login_display_name;

-- users: Password history and change timestamp
ALTER TABLE users
    DROP COLUMN IF EXISTS password_history,
    DROP COLUMN IF EXISTS password_changed_at;
