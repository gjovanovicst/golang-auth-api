-- Rollback: Remove settings:read and settings:write from the member role
-- Reverts migration 20260317_add_settings_permissions_to_member.

BEGIN;

-- Remove schema_migrations tracking record
DELETE FROM schema_migrations
WHERE version = '20260317_add_settings_permissions_to_member';

-- Remove settings:read from the system member role in every application
DELETE FROM role_permissions
WHERE role_id IN (
    SELECT id FROM roles WHERE name = 'member' AND is_system = TRUE
)
AND permission_id IN (
    SELECT id FROM permissions WHERE resource = 'settings' AND action = 'read'
);

-- Remove settings:write from the system member role in every application
DELETE FROM role_permissions
WHERE role_id IN (
    SELECT id FROM roles WHERE name = 'member' AND is_system = TRUE
)
AND permission_id IN (
    SELECT id FROM permissions WHERE resource = 'settings' AND action = 'write'
);

COMMIT;
