-- Step 0: Enforce global email uniqueness in Auth API
-- One Email = One Global_User_UUID. This is the bedrock of the entire identity system.
-- Previously email uniqueness was per-app (composite index on email + app_id).
-- Now email must be globally unique across all apps.

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email_global ON users(email);

-- Record Migration
INSERT INTO schema_migrations (version, name, applied_at, success)
VALUES ('20260508_add_global_email_unique', 'Enforce global email uniqueness across all apps', NOW(), true)
ON CONFLICT (version) DO NOTHING;
