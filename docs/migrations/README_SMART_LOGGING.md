# Activity Log Smart Logging Migration

## Overview

This migration adds professional activity logging features to reduce database bloat while maintaining security audit capabilities.

## What Changed

### New Fields Added to `activity_logs` Table

1. **severity** (VARCHAR(20), NOT NULL, DEFAULT 'INFORMATIONAL')
   - Values: CRITICAL, IMPORTANT, INFORMATIONAL
   - Categorizes events by security importance

2. **expires_at** (TIMESTAMP WITH TIME ZONE, NULLABLE)
   - Automatic expiration timestamp based on retention policies
   - Enables efficient cleanup of old logs

3. **is_anomaly** (BOOLEAN, NOT NULL, DEFAULT false)
   - Flags logs created due to anomaly detection
   - Helps identify security-relevant events

### New Indexes

- `idx_activity_logs_expires` - Speeds up cleanup queries
- `idx_activity_logs_cleanup` - Composite index for batch cleanup
- `idx_activity_logs_user_timestamp` - Optimizes user activity pattern queries

## How to Apply

### Using psql (Manual)

```bash
psql -U your_user -d your_database -f migrations/20240103_add_activity_log_smart_fields.sql
```

### Using Golang (Programmatic)

The migration will be applied automatically when the application starts if using GORM AutoMigrate on the updated `ActivityLog` model.

## Rollback

If you need to rollback this migration:

```bash
psql -U your_user -d your_database -f migrations/20240103_add_activity_log_smart_fields_rollback.sql
```

## Expected Impact

### Database Size Reduction
- **Before**: All events logged indefinitely (TOKEN_REFRESH, PROFILE_ACCESS, etc.)
- **After**: 80-95% reduction through:
  - Disabled high-frequency events (TOKEN_REFRESH, PROFILE_ACCESS) by default
  - Automatic cleanup based on retention policies
  - Anomaly-based conditional logging

### Retention Policies
- **Critical Events**: 365 days (LOGIN, PASSWORD_CHANGE, 2FA_ENABLE, etc.)
- **Important Events**: 180 days (EMAIL_VERIFY, SOCIAL_LOGIN, etc.)
- **Informational Events**: 90 days (TOKEN_REFRESH, PROFILE_ACCESS if enabled)

## Configuration

Set these environment variables to customize behavior:

```bash
# Enable/disable cleanup (default: true)
LOG_CLEANUP_ENABLED=true

# Cleanup interval (default: 24h)
LOG_CLEANUP_INTERVAL=24h

# Batch size for cleanup operations (default: 1000)
LOG_CLEANUP_BATCH_SIZE=1000

# Disable specific events (comma-separated)
LOG_DISABLED_EVENTS=TOKEN_REFRESH,PROFILE_ACCESS

# Enable anomaly detection (default: true)
LOG_ANOMALY_DETECTION_ENABLED=true

# Custom retention periods (in days)
LOG_RETENTION_CRITICAL=365
LOG_RETENTION_IMPORTANT=180
LOG_RETENTION_INFORMATIONAL=90
```

## Verification

After applying the migration, verify the changes:

```sql
-- Check new columns exist
\d activity_logs

-- Check indexes
\di activity_logs*

-- Verify severity distribution
SELECT severity, COUNT(*) 
FROM activity_logs 
GROUP BY severity;

-- Check expiration dates are set
SELECT 
    severity,
    COUNT(*) as total,
    COUNT(expires_at) as with_expiration,
    MIN(expires_at) as earliest_expiration,
    MAX(expires_at) as latest_expiration
FROM activity_logs
GROUP BY severity;
```

## Troubleshooting

### Migration Fails

If the migration fails due to existing data:

1. Backup your database first
2. Check for NULL values or invalid data
3. Run the migration steps individually to identify the issue

### Performance Issues

If cleanup is slow:

1. Adjust `LOG_CLEANUP_BATCH_SIZE` to a smaller value
2. Increase `LOG_CLEANUP_INTERVAL` to reduce frequency
3. Consider running cleanup during off-peak hours

### Too Much Data Being Deleted

If cleanup is too aggressive:

1. Increase retention periods via environment variables
2. Change event severities in code to keep them longer
3. Disable cleanup temporarily while investigating

## Support

For issues or questions, refer to:
- Main documentation: `docs/API.md`
- Implementation plan: `professional.plan.md`
- Code: `internal/log/` and `internal/config/logging.go`

