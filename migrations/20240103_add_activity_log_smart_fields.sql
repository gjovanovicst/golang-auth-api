-- Migration: Add smart logging fields to activity_logs table
-- Date: 2024-01-03
-- Description: Adds severity, expires_at, and is_anomaly fields for professional activity logging

-- Add new columns
ALTER TABLE activity_logs 
ADD COLUMN IF NOT EXISTS severity VARCHAR(20) NOT NULL DEFAULT 'INFORMATIONAL',
ADD COLUMN IF NOT EXISTS expires_at TIMESTAMP WITH TIME ZONE,
ADD COLUMN IF NOT EXISTS is_anomaly BOOLEAN NOT NULL DEFAULT false;

-- Add comments for documentation
COMMENT ON COLUMN activity_logs.severity IS 'Event severity: CRITICAL, IMPORTANT, or INFORMATIONAL';
COMMENT ON COLUMN activity_logs.expires_at IS 'Automatic expiration timestamp for log cleanup based on retention policies';
COMMENT ON COLUMN activity_logs.is_anomaly IS 'Flag indicating if this log was created due to anomaly detection';

-- Create indexes for efficient cleanup queries
CREATE INDEX IF NOT EXISTS idx_activity_logs_expires ON activity_logs(expires_at) WHERE expires_at IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_activity_logs_cleanup ON activity_logs(severity, expires_at, timestamp);

-- Create composite index for user activity pattern queries
CREATE INDEX IF NOT EXISTS idx_activity_logs_user_timestamp ON activity_logs(user_id, timestamp DESC);

-- Update existing records to set severity based on event type
UPDATE activity_logs SET severity = 'CRITICAL' 
WHERE event_type IN (
    'LOGIN', 'LOGOUT', 'REGISTER', 'PASSWORD_CHANGE', 'PASSWORD_RESET',
    'EMAIL_CHANGE', '2FA_ENABLE', '2FA_DISABLE', 'ACCOUNT_DELETION', 'RECOVERY_CODE_USED'
);

UPDATE activity_logs SET severity = 'IMPORTANT' 
WHERE event_type IN (
    'EMAIL_VERIFY', '2FA_LOGIN', 'SOCIAL_LOGIN', 'PROFILE_UPDATE', 'RECOVERY_CODE_GENERATE'
);

UPDATE activity_logs SET severity = 'INFORMATIONAL' 
WHERE event_type IN (
    'TOKEN_REFRESH', 'PROFILE_ACCESS'
);

-- Set expiration dates for existing records based on severity
-- Critical: 365 days (1 year)
UPDATE activity_logs SET expires_at = timestamp + INTERVAL '365 days' 
WHERE severity = 'CRITICAL' AND expires_at IS NULL;

-- Important: 180 days (6 months)
UPDATE activity_logs SET expires_at = timestamp + INTERVAL '180 days' 
WHERE severity = 'IMPORTANT' AND expires_at IS NULL;

-- Informational: 90 days (3 months)
UPDATE activity_logs SET expires_at = timestamp + INTERVAL '90 days' 
WHERE severity = 'INFORMATIONAL' AND expires_at IS NULL;

-- Add constraint to ensure valid severity values (if not exists)
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint 
        WHERE conname = 'chk_activity_logs_severity'
    ) THEN
        ALTER TABLE activity_logs 
        ADD CONSTRAINT chk_activity_logs_severity 
        CHECK (severity IN ('CRITICAL', 'IMPORTANT', 'INFORMATIONAL'));
    END IF;
END $$;

