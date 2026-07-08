-- Rollback: Remove user.deleted webhook event type and archived_users table
-- Date: 2026-06-25

BEGIN;

-- Drop the archived_users table
DROP TABLE IF EXISTS archived_users;

-- Revert CHECK constraint to original 8 event types (without user.deleted)
ALTER TABLE webhook_endpoints DROP CONSTRAINT IF EXISTS chk_webhook_event_type;

ALTER TABLE webhook_endpoints ADD CONSTRAINT chk_webhook_event_type CHECK (event_type IN (
    'user.registered',
    'user.verified',
    'user.login',
    'user.password_changed',
    '2fa.enabled',
    '2fa.disabled',
    'social.linked',
    'social.unlinked'
));

-- Remove migration record
DELETE FROM schema_migrations WHERE version = '20260625_add_user_deleted_webhook_and_archive';

COMMIT;
