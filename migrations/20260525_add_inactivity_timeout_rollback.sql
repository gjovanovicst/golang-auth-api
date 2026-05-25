-- Rollback: 20260525_add_inactivity_timeout
ALTER TABLE applications
    DROP COLUMN IF EXISTS inactivity_timeout_minutes;
