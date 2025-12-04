-- Activity Logs Cleanup Script
-- WARNING: These operations are DESTRUCTIVE and CANNOT be undone
-- Always backup your database before running cleanup operations

-- ============================================================================
-- OPTION 1: Delete ALL activity logs (DANGEROUS - Use with caution!)
-- ============================================================================
-- Uncomment to execute:
-- DELETE FROM activity_logs;

-- Verify deletion:
-- SELECT COUNT(*) FROM activity_logs;


-- ============================================================================
-- OPTION 2: Delete logs older than a specific date
-- ============================================================================
-- Delete logs older than 30 days
-- DELETE FROM activity_logs WHERE timestamp < NOW() - INTERVAL '30 days';

-- Delete logs older than 90 days
-- DELETE FROM activity_logs WHERE timestamp < NOW() - INTERVAL '90 days';

-- Delete logs older than 1 year
-- DELETE FROM activity_logs WHERE timestamp < NOW() - INTERVAL '1 year';


-- ============================================================================
-- OPTION 3: Delete by severity (keep critical, remove informational)
-- ============================================================================
-- Delete only informational logs
-- DELETE FROM activity_logs WHERE severity = 'INFORMATIONAL';

-- Delete informational and important logs (keep only critical)
-- DELETE FROM activity_logs WHERE severity IN ('INFORMATIONAL', 'IMPORTANT');


-- ============================================================================
-- OPTION 4: Delete expired logs (recommended - follows retention policies)
-- ============================================================================
-- This is what the automatic cleanup service does
-- DELETE FROM activity_logs WHERE expires_at < NOW();


-- ============================================================================
-- OPTION 5: Delete specific event types
-- ============================================================================
-- Delete only TOKEN_REFRESH logs
-- DELETE FROM activity_logs WHERE event_type = 'TOKEN_REFRESH';

-- Delete TOKEN_REFRESH and PROFILE_ACCESS logs
-- DELETE FROM activity_logs WHERE event_type IN ('TOKEN_REFRESH', 'PROFILE_ACCESS');


-- ============================================================================
-- OPTION 6: Delete logs for specific users (GDPR right to be forgotten)
-- ============================================================================
-- Replace 'user-uuid-here' with actual user ID
-- DELETE FROM activity_logs WHERE user_id = 'user-uuid-here';


-- ============================================================================
-- OPTION 7: Keep only recent logs (e.g., last 7 days)
-- ============================================================================
-- Keep only logs from last 7 days, delete everything else
-- DELETE FROM activity_logs WHERE timestamp < NOW() - INTERVAL '7 days';


-- ============================================================================
-- OPTION 8: Batch deletion (for very large tables - prevents locking)
-- ============================================================================
-- Delete in batches of 10000 rows at a time
-- Run this multiple times until it returns 0 rows affected

-- DO $$
-- DECLARE
--     deleted_count INTEGER;
-- BEGIN
--     LOOP
--         DELETE FROM activity_logs 
--         WHERE id IN (
--             SELECT id FROM activity_logs 
--             WHERE expires_at < NOW() 
--             LIMIT 10000
--         );
--         
--         GET DIAGNOSTICS deleted_count = ROW_COUNT;
--         
--         RAISE NOTICE 'Deleted % rows', deleted_count;
--         
--         EXIT WHEN deleted_count = 0;
--         
--         -- Small delay between batches
--         PERFORM pg_sleep(0.1);
--     END LOOP;
-- END $$;


-- ============================================================================
-- OPTION 9: Archive before deletion (export to CSV)
-- ============================================================================
-- Export to CSV before deletion
-- \copy (SELECT * FROM activity_logs) TO '/path/to/backup/activity_logs_backup.csv' CSV HEADER;
-- 
-- Then delete:
-- DELETE FROM activity_logs;


-- ============================================================================
-- USEFUL QUERIES - Check before deleting
-- ============================================================================

-- Count total logs
SELECT COUNT(*) as total_logs FROM activity_logs;

-- Count by severity
SELECT severity, COUNT(*) as count 
FROM activity_logs 
GROUP BY severity 
ORDER BY severity;

-- Count by event type
SELECT event_type, COUNT(*) as count 
FROM activity_logs 
GROUP BY event_type 
ORDER BY count DESC 
LIMIT 20;

-- Check oldest and newest logs
SELECT 
    MIN(timestamp) as oldest_log,
    MAX(timestamp) as newest_log,
    COUNT(*) as total_logs
FROM activity_logs;

-- Count expired logs (ready for cleanup)
SELECT COUNT(*) as expired_logs 
FROM activity_logs 
WHERE expires_at < NOW();

-- Count logs by time period
SELECT 
    CASE 
        WHEN timestamp > NOW() - INTERVAL '7 days' THEN 'Last 7 days'
        WHEN timestamp > NOW() - INTERVAL '30 days' THEN 'Last 30 days'
        WHEN timestamp > NOW() - INTERVAL '90 days' THEN 'Last 90 days'
        WHEN timestamp > NOW() - INTERVAL '365 days' THEN 'Last year'
        ELSE 'Over 1 year old'
    END as age_group,
    COUNT(*) as count
FROM activity_logs
GROUP BY age_group
ORDER BY 
    CASE age_group
        WHEN 'Last 7 days' THEN 1
        WHEN 'Last 30 days' THEN 2
        WHEN 'Last 90 days' THEN 3
        WHEN 'Last year' THEN 4
        ELSE 5
    END;

-- Estimate table size
SELECT 
    pg_size_pretty(pg_total_relation_size('activity_logs')) as total_size,
    pg_size_pretty(pg_relation_size('activity_logs')) as table_size,
    pg_size_pretty(pg_indexes_size('activity_logs')) as indexes_size;


-- ============================================================================
-- POST-CLEANUP MAINTENANCE
-- ============================================================================

-- After large deletions, reclaim space and update statistics
-- VACUUM ANALYZE activity_logs;

-- For aggressive space reclamation (locks table briefly)
-- VACUUM FULL activity_logs;

-- Reindex for optimal performance
-- REINDEX TABLE activity_logs;

