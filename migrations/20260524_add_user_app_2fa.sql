-- Migration: 20260524_add_user_app_2fa
-- Description: Per-application 2FA configuration for users.
--              Each (user_id, application_id) pair has its own TOTP secret,
--              recovery codes, and enabled flag. Existing users with global
--              2FA enabled are migrated: a row is created for every application
--              they belong to, copying their current secret/method/recovery codes.
--              All other (user, app) combinations get a row with two_fa_enabled=false
--              so the fallback to the global flag is never needed.

-- 1. Create the table
CREATE TABLE user_app_2fa (
    id              UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    user_id         UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    application_id  UUID        NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    two_fa_enabled  BOOLEAN     NOT NULL DEFAULT false,
    two_fa_method   VARCHAR(20) NOT NULL DEFAULT '',
    two_fa_secret   TEXT        NOT NULL DEFAULT '',
    recovery_codes  JSONB,
    previous_method VARCHAR(20) NOT NULL DEFAULT '',
    previous_secret TEXT        NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT idx_user_app_2fa UNIQUE (user_id, application_id)
);

CREATE INDEX idx_user_app_2fa_user_id        ON user_app_2fa (user_id);
CREATE INDEX idx_user_app_2fa_application_id ON user_app_2fa (application_id);

-- 2. Data migration: for users with 2FA enabled, copy their config to the apps
--    they belong to (registered app or user_apps). For all other (user, app)
--    combinations, insert a disabled row so the legacy fallback is never hit.

-- Step A: insert enabled rows for apps the user belongs to
INSERT INTO user_app_2fa (
    user_id,
    application_id,
    two_fa_enabled,
    two_fa_method,
    two_fa_secret,
    recovery_codes,
    previous_method,
    previous_secret
)
SELECT DISTINCT
    u.id                    AS user_id,
    apps.application_id     AS application_id,
    u.two_fa_enabled        AS two_fa_enabled,
    COALESCE(NULLIF(u.two_fa_method, ''), 'totp') AS two_fa_method,
    COALESCE(u.two_fa_secret, '')                 AS two_fa_secret,
    u.two_fa_recovery_codes                       AS recovery_codes,
    COALESCE(u.two_fa_previous_method, '')        AS previous_method,
    COALESCE(u.two_fa_previous_secret, '')        AS previous_secret
FROM users u
-- Build the set of application IDs each user is associated with:
-- (a) the app the user originally registered with
JOIN (
    SELECT u2.id AS user_id, u2.app_id AS application_id
    FROM users u2
    UNION
    -- (b) any additional app the user belongs to (via user_apps join table)
    SELECT ua.user_id, ua.app_id AS application_id
    FROM user_apps ua
) apps ON apps.user_id = u.id
WHERE u.two_fa_enabled = true
ON CONFLICT (user_id, application_id) DO NOTHING;

-- Step B: for every remaining (user, app) pair that has no row yet, insert disabled.
--         This covers apps where the user has no prior relationship (e.g. Permissio).
INSERT INTO user_app_2fa (user_id, application_id, two_fa_enabled)
SELECT u.id, a.id, false
FROM users u
CROSS JOIN applications a
ON CONFLICT (user_id, application_id) DO NOTHING;
