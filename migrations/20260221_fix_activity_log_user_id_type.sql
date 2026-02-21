-- Fix activity_logs.user_id column type from text to uuid
-- This migration corrects a GORM auto-migrate issue where the column was created
-- without an explicit type:uuid tag, causing "operator does not exist: uuid = text"
-- errors on JOIN queries.

ALTER TABLE activity_logs
    ALTER COLUMN user_id TYPE uuid USING user_id::uuid;
