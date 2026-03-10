-- Rollback: remove two_fa_previous_method and two_fa_previous_secret columns from users

ALTER TABLE users
    DROP COLUMN IF EXISTS two_fa_previous_method,
    DROP COLUMN IF EXISTS two_fa_previous_secret;

DELETE FROM schema_migrations WHERE version = '20260310_add_two_fa_previous_method';
