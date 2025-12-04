-- Rollback Migration: Remove smart logging fields from activity_logs table
-- Date: 2024-01-03
-- Description: Rolls back the addition of severity, expires_at, and is_anomaly fields

-- Drop constraint
ALTER TABLE activity_logs DROP CONSTRAINT IF EXISTS chk_activity_logs_severity;

-- Drop indexes
DROP INDEX IF EXISTS idx_activity_logs_expires;
DROP INDEX IF EXISTS idx_activity_logs_cleanup;
DROP INDEX IF EXISTS idx_activity_logs_user_timestamp;

-- Drop columns
ALTER TABLE activity_logs 
DROP COLUMN IF EXISTS severity,
DROP COLUMN IF EXISTS expires_at,
DROP COLUMN IF EXISTS is_anomaly;

