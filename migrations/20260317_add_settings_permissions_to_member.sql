-- Migration: Grant settings:read and settings:write to the member role
-- The member role is the default role assigned to all new users. Without
-- settings:read and settings:write, members receive 403 on every 2FA
-- management endpoint (TOTP, email 2FA, SMS 2FA, backup email, passkeys,
-- trusted devices, phone management). This migration fixes that gap.

BEGIN;

-- Track this migration
INSERT INTO schema_migrations (version, name, applied_at)
VALUES ('20260317_add_settings_permissions_to_member', '20260317_add_settings_permissions_to_member', NOW())
ON CONFLICT (version) DO NOTHING;

-- Grant settings:read to the system member role in every application
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
CROSS JOIN permissions p
WHERE r.name = 'member'
  AND r.is_system = TRUE
  AND p.resource = 'settings'
  AND p.action = 'read'
ON CONFLICT DO NOTHING;

-- Grant settings:write to the system member role in every application
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
CROSS JOIN permissions p
WHERE r.name = 'member'
  AND r.is_system = TRUE
  AND p.resource = 'settings'
  AND p.action = 'write'
ON CONFLICT DO NOTHING;

COMMIT;
