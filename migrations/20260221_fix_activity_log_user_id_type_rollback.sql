-- Rollback: revert activity_logs.user_id column type back to text
ALTER TABLE activity_logs
    ALTER COLUMN user_id TYPE text USING user_id::text;
