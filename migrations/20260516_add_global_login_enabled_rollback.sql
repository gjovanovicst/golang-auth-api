-- Rollback: remove global_login_enabled flag from applications table.
ALTER TABLE applications DROP COLUMN IF EXISTS global_login_enabled;
