# Activity Logging Configuration Guide

## Overview

The authentication API implements a professional, tiered activity logging system that balances security audit requirements with database performance and storage costs.

## Key Features

### 1. Event Severity Classification
Events are categorized by security importance:
- **Critical**: Authentication changes, security events (1-year retention)
- **Important**: Verification events, profile changes (6-month retention)
- **Informational**: Routine operations (3-month retention)

### 2. Smart Logging
- High-frequency events (TOKEN_REFRESH, PROFILE_ACCESS) are disabled by default
- Anomaly detection logs unusual patterns even for disabled events
- Configurable sampling for enabled informational events

### 3. Automatic Cleanup
- Background service runs daily to delete expired logs
- Retention policies based on event severity
- Graceful batch processing to avoid database locks

### 4. Anomaly Detection
Logs informational events when unusual patterns detected:
- First access from a new IP address
- New device or browser (user agent change)
- Access from unusual times (optional)

## Default Behavior

Out of the box, the system:
- ✅ Logs all critical and important events
- ❌ Does NOT log TOKEN_REFRESH (happens every 15 minutes)
- ❌ Does NOT log PROFILE_ACCESS (happens on every profile view)
- ✅ DOES log TOKEN_REFRESH/PROFILE_ACCESS if anomaly detected
- ✅ Automatically cleans up logs older than retention period
- ✅ Runs cleanup daily at midnight

## Configuration Options

### Environment Variables

#### Basic Event Control

```bash
# Completely disable specific events (comma-separated)
LOG_DISABLED_EVENTS=TOKEN_REFRESH,PROFILE_ACCESS

# Or enable specific informational events
LOG_TOKEN_REFRESH=true      # Enable token refresh logging
LOG_PROFILE_ACCESS=true     # Enable profile access logging
```

#### Sampling Configuration

When informational events are enabled, control how many get logged:

```bash
# Log only 1% of token refreshes (reduces 99% of log entries)
LOG_SAMPLE_TOKEN_REFRESH=0.01

# Log only 5% of profile accesses
LOG_SAMPLE_PROFILE_ACCESS=0.05

# Log 100% (all) - same as no sampling
LOG_SAMPLE_TOKEN_REFRESH=1.0
```

#### Anomaly Detection

```bash
# Enable/disable anomaly detection entirely
LOG_ANOMALY_DETECTION_ENABLED=true

# Control what triggers anomaly logging
LOG_ANOMALY_NEW_IP=true              # Log when user accesses from new IP
LOG_ANOMALY_NEW_USER_AGENT=true      # Log when user uses new device/browser
LOG_ANOMALY_UNUSUAL_TIME=false       # Log unusual time access (requires more data)

# How long to remember user patterns (default: 30 days)
LOG_ANOMALY_SESSION_WINDOW=720h      # 720 hours = 30 days
```

#### Retention Policies

```bash
# Customize how long logs are kept (in days)
LOG_RETENTION_CRITICAL=365           # Critical events: 1 year
LOG_RETENTION_IMPORTANT=180          # Important events: 6 months
LOG_RETENTION_INFORMATIONAL=90       # Informational events: 3 months

# Example: Keep critical events for 2 years
LOG_RETENTION_CRITICAL=730
```

#### Cleanup Service

```bash
# Enable/disable automatic cleanup
LOG_CLEANUP_ENABLED=true

# How often to run cleanup (Go duration format)
LOG_CLEANUP_INTERVAL=24h             # Every 24 hours
LOG_CLEANUP_INTERVAL=12h             # Every 12 hours
LOG_CLEANUP_INTERVAL=168h            # Every week

# Batch size for cleanup operations
LOG_CLEANUP_BATCH_SIZE=1000          # Delete 1000 logs per batch
LOG_CLEANUP_BATCH_SIZE=5000          # Larger batches (faster but more DB load)
```

## Use Case Examples

### High Security Environment

Log everything, keep it longer:

```bash
# Log all events including informational
LOG_TOKEN_REFRESH=true
LOG_PROFILE_ACCESS=true

# But sample them to reduce volume
LOG_SAMPLE_TOKEN_REFRESH=0.1         # Log 10%
LOG_SAMPLE_PROFILE_ACCESS=0.1

# Keep logs longer
LOG_RETENTION_CRITICAL=730           # 2 years
LOG_RETENTION_IMPORTANT=365          # 1 year
LOG_RETENTION_INFORMATIONAL=180      # 6 months

# Aggressive anomaly detection
LOG_ANOMALY_UNUSUAL_TIME=true
```

### High Traffic / Low Storage

Minimize logging, aggressive cleanup:

```bash
# Only log critical events
LOG_DISABLED_EVENTS=PROFILE_ACCESS,TOKEN_REFRESH,EMAIL_VERIFY,PROFILE_UPDATE

# Shorter retention
LOG_RETENTION_CRITICAL=180           # 6 months
LOG_RETENTION_IMPORTANT=90           # 3 months
LOG_RETENTION_INFORMATIONAL=30       # 1 month

# More frequent cleanup
LOG_CLEANUP_INTERVAL=6h              # Every 6 hours
LOG_CLEANUP_BATCH_SIZE=5000
```

### Development Environment

Log everything for debugging:

```bash
# Enable all events
LOG_TOKEN_REFRESH=true
LOG_PROFILE_ACCESS=true

# No sampling (log 100%)
LOG_SAMPLE_TOKEN_REFRESH=1.0
LOG_SAMPLE_PROFILE_ACCESS=1.0

# Disable cleanup (manual cleanup)
LOG_CLEANUP_ENABLED=false

# Short retention for automatic cleanup if enabled
LOG_RETENTION_CRITICAL=7
LOG_RETENTION_IMPORTANT=7
LOG_RETENTION_INFORMATIONAL=7
```

### GDPR Compliance

Focus on minimal data collection:

```bash
# Disable informational events
LOG_DISABLED_EVENTS=TOKEN_REFRESH,PROFILE_ACCESS

# Disable anomaly detection (less tracking)
LOG_ANOMALY_DETECTION_ENABLED=false

# Aggressive cleanup
LOG_RETENTION_CRITICAL=90
LOG_RETENTION_IMPORTANT=60
LOG_RETENTION_INFORMATIONAL=30
```

## Database Schema

The `activity_logs` table includes these fields:

```sql
CREATE TABLE activity_logs (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL,
    event_type VARCHAR(50) NOT NULL,
    timestamp TIMESTAMP WITH TIME ZONE NOT NULL,
    ip_address VARCHAR(45),
    user_agent TEXT,
    details JSONB,
    
    -- Smart logging fields
    severity VARCHAR(20) NOT NULL DEFAULT 'INFORMATIONAL',
    expires_at TIMESTAMP WITH TIME ZONE,
    is_anomaly BOOLEAN NOT NULL DEFAULT false
);
```

## Monitoring and Maintenance

### Check Cleanup Status

Use the database to check cleanup effectiveness:

```sql
-- Count logs by severity
SELECT severity, COUNT(*) as count
FROM activity_logs
GROUP BY severity;

-- Check logs near expiration
SELECT severity, COUNT(*) as expiring_soon
FROM activity_logs
WHERE expires_at < NOW() + INTERVAL '7 days'
GROUP BY severity;

-- Count expired logs waiting for cleanup
SELECT COUNT(*) as expired_count
FROM activity_logs
WHERE expires_at < NOW();
```

### Manual Cleanup

If needed, trigger manual cleanup:

```sql
-- Delete expired logs manually
DELETE FROM activity_logs
WHERE expires_at < NOW();

-- Delete logs for specific user (GDPR right to be forgotten)
DELETE FROM activity_logs
WHERE user_id = 'user-uuid-here';
```

### Performance Monitoring

Monitor these metrics:
- Database size growth rate
- Cleanup execution time
- Query performance on activity_logs table
- Anomaly detection rate

## Migration

See `migrations/README_SMART_LOGGING.md` for detailed migration instructions.

### Quick Start

1. Apply the database migration:
   ```bash
   psql -U your_user -d your_database -f migrations/20240103_add_activity_log_smart_fields.sql
   ```

2. Restart the application (it will use the new logging system automatically)

3. Configure via environment variables if needed

4. Monitor database size and adjust settings as needed

## Troubleshooting

### Too Many Logs Being Created

**Problem**: Database growing too fast
**Solutions**:
- Disable more events: `LOG_DISABLED_EVENTS=TOKEN_REFRESH,PROFILE_ACCESS,PROFILE_UPDATE`
- Increase sampling rates: `LOG_SAMPLE_TOKEN_REFRESH=0.001` (0.1%)
- Reduce retention: `LOG_RETENTION_INFORMATIONAL=30`

### Not Enough Security Audit Data

**Problem**: Missing important security events
**Solutions**:
- Enable anomaly detection: `LOG_ANOMALY_DETECTION_ENABLED=true`
- Enable specific events: `LOG_TOKEN_REFRESH=true` with sampling
- Increase retention: `LOG_RETENTION_CRITICAL=730` (2 years)

### Cleanup Not Working

**Problem**: Old logs not being deleted
**Solutions**:
- Check cleanup is enabled: `LOG_CLEANUP_ENABLED=true`
- Verify application is running continuously
- Check database logs for errors
- Manually run cleanup query to test permissions

### High Anomaly Rate

**Problem**: Too many anomalies being detected
**Solutions**:
- Increase session window: `LOG_ANOMALY_SESSION_WINDOW=1440h` (60 days)
- Disable specific anomaly checks: `LOG_ANOMALY_UNUSUAL_TIME=false`
- Review user behavior patterns (mobile users, VPNs, etc.)

## Best Practices

1. **Start Conservative**: Begin with default settings and monitor
2. **Tune Gradually**: Adjust based on actual traffic patterns
3. **Monitor First**: Watch database size for a week before changing retention
4. **Security First**: Don't disable critical events to save storage
5. **Test Cleanup**: Verify cleanup works before relying on it
6. **Document Changes**: Keep track of configuration changes
7. **Review Regularly**: Audit logging configuration quarterly

## Support

For issues or questions:
- Review this guide and `docs/API.md`
- Check `migrations/README_SMART_LOGGING.md` for migration help
- Review code: `internal/log/` and `internal/config/logging.go`
- Examine the plan: `professional.plan.md`

