-- Rollback: RBAC Seed Migration

BEGIN;

-- Remove role-permission assignments for system roles
DELETE FROM role_permissions
WHERE role_id IN (SELECT id FROM roles WHERE is_system = TRUE);

-- Remove system roles
DELETE FROM roles WHERE is_system = TRUE;

-- Remove seeded permissions
DELETE FROM permissions WHERE id IN (
    'a0000000-0000-0000-0000-000000000001',
    'a0000000-0000-0000-0000-000000000002',
    'a0000000-0000-0000-0000-000000000003',
    'a0000000-0000-0000-0000-000000000004',
    'a0000000-0000-0000-0000-000000000005',
    'a0000000-0000-0000-0000-000000000006',
    'a0000000-0000-0000-0000-000000000007',
    'a0000000-0000-0000-0000-000000000008',
    'a0000000-0000-0000-0000-000000000009',
    'a0000000-0000-0000-0000-000000000010'
);

DELETE FROM schema_migrations WHERE version = '20260301_seed_rbac_defaults';

COMMIT;
