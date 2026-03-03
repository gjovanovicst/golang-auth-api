-- RBAC Seed Migration: Default permissions and roles for all existing applications
-- Permissions are global. Roles are created per-app with is_system=true.

BEGIN;

-- Track this migration
INSERT INTO schema_migrations (version, name, applied_at)
VALUES ('20260301_seed_rbac_defaults', '20260301_seed_rbac_defaults', NOW())
ON CONFLICT (version) DO NOTHING;

-- ============================================================
-- 1. Seed default permissions (global, not per-app)
-- ============================================================
INSERT INTO permissions (id, resource, action, description) VALUES
    ('a0000000-0000-0000-0000-000000000001', 'user',       'read',   'View user profiles and information'),
    ('a0000000-0000-0000-0000-000000000002', 'user',       'write',  'Create and update user information'),
    ('a0000000-0000-0000-0000-000000000003', 'user',       'delete', 'Delete user accounts'),
    ('a0000000-0000-0000-0000-000000000004', 'log',        'read',   'View activity logs'),
    ('a0000000-0000-0000-0000-000000000005', 'log',        'delete', 'Delete activity log entries'),
    ('a0000000-0000-0000-0000-000000000006', 'settings',   'read',   'View application settings'),
    ('a0000000-0000-0000-0000-000000000007', 'settings',   'write',  'Modify application settings'),
    ('a0000000-0000-0000-0000-000000000008', 'role',       'read',   'View roles and permissions'),
    ('a0000000-0000-0000-0000-000000000009', 'role',       'write',  'Create, update, and delete roles'),
    ('a0000000-0000-0000-0000-000000000010', 'role',       'assign', 'Assign and revoke roles for users')
ON CONFLICT (resource, action) DO NOTHING;

-- ============================================================
-- 2. Create default roles for EACH existing application
-- ============================================================
-- Admin role: all permissions
INSERT INTO roles (id, app_id, name, description, is_system, created_at, updated_at)
SELECT
    gen_random_uuid(),
    a.id,
    'admin',
    'Full access to all resources within the application',
    TRUE,
    NOW(),
    NOW()
FROM applications a
WHERE NOT EXISTS (
    SELECT 1 FROM roles r WHERE r.app_id = a.id AND r.name = 'admin'
);

-- Member role: standard user access
INSERT INTO roles (id, app_id, name, description, is_system, created_at, updated_at)
SELECT
    gen_random_uuid(),
    a.id,
    'member',
    'Standard user with read and limited write access',
    TRUE,
    NOW(),
    NOW()
FROM applications a
WHERE NOT EXISTS (
    SELECT 1 FROM roles r WHERE r.app_id = a.id AND r.name = 'member'
);

-- Viewer role: read-only access
INSERT INTO roles (id, app_id, name, description, is_system, created_at, updated_at)
SELECT
    gen_random_uuid(),
    a.id,
    'viewer',
    'Read-only access to resources',
    TRUE,
    NOW(),
    NOW()
FROM applications a
WHERE NOT EXISTS (
    SELECT 1 FROM roles r WHERE r.app_id = a.id AND r.name = 'viewer'
);

-- ============================================================
-- 3. Assign permissions to roles
-- ============================================================

-- Admin gets ALL permissions
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
CROSS JOIN permissions p
WHERE r.name = 'admin' AND r.is_system = TRUE
ON CONFLICT DO NOTHING;

-- Member gets: user:read, user:write, log:read, role:read
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
CROSS JOIN permissions p
WHERE r.name = 'member' AND r.is_system = TRUE
  AND (
    (p.resource = 'user' AND p.action IN ('read', 'write'))
    OR (p.resource = 'log' AND p.action = 'read')
    OR (p.resource = 'role' AND p.action = 'read')
  )
ON CONFLICT DO NOTHING;

-- Viewer gets: user:read, log:read, role:read
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
CROSS JOIN permissions p
WHERE r.name = 'viewer' AND r.is_system = TRUE
  AND (
    (p.resource = 'user' AND p.action = 'read')
    OR (p.resource = 'log' AND p.action = 'read')
    OR (p.resource = 'role' AND p.action = 'read')
  )
ON CONFLICT DO NOTHING;

COMMIT;
