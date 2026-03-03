-- Rollback: RBAC Schema Migration

BEGIN;

DROP TABLE IF EXISTS user_roles;
DROP TABLE IF EXISTS role_permissions;
DROP TABLE IF EXISTS roles;
DROP TABLE IF EXISTS permissions;

DELETE FROM schema_migrations WHERE version = '20260301_add_rbac';

COMMIT;
