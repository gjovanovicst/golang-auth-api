-- Rollback: Remove backfilled "member" role assignments
-- Only removes assignments where assigned_by IS NULL (i.e., backfilled by this migration).
-- This avoids removing role assignments made by admins or via the application.

BEGIN;

-- Remove backfilled member role assignments (those with no assigned_by)
DELETE FROM user_roles
WHERE assigned_by IS NULL
  AND role_id IN (
    SELECT id FROM roles WHERE name = 'member' AND is_system = TRUE
  );

DELETE FROM schema_migrations WHERE version = '20260302_backfill_member_role';

COMMIT;
