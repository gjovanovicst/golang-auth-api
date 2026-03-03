-- Backfill Migration: Assign "member" role to all existing users without any role
-- This fixes the "Insufficient permissions" bug for users registered before RBAC was added.
-- The seed migration (20260301_seed_rbac_defaults) created roles and permissions but did
-- not assign the "member" role to existing users in the user_roles table.

BEGIN;

-- Track this migration
INSERT INTO schema_migrations (version, name, applied_at)
VALUES ('20260302_backfill_member_role', '20260302_backfill_member_role', NOW())
ON CONFLICT (version) DO NOTHING;

-- Assign the "member" role to every user who currently has NO role assignment.
-- Joins users to the "member" role in their app, excluding users already in user_roles.
INSERT INTO user_roles (user_id, role_id, app_id, assigned_at, assigned_by)
SELECT
    u.id,
    r.id,
    u.app_id,
    NOW(),
    NULL
FROM users u
INNER JOIN roles r ON r.app_id = u.app_id AND r.name = 'member' AND r.is_system = TRUE
WHERE NOT EXISTS (
    SELECT 1 FROM user_roles ur WHERE ur.user_id = u.id
);

COMMIT;
