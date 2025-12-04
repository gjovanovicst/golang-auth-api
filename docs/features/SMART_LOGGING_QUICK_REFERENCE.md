# Activity Logging Quick Reference

## TL;DR

The new activity logging system reduces database bloat by 80-95% while maintaining security audit capabilities. High-frequency events are disabled by default but logged when anomalies detected.

## Quick Start

### Zero Configuration Required
The system works out-of-the-box with smart defaults:
- ✅ Critical security events always logged
- ❌ High-frequency events (TOKEN_REFRESH, PROFILE_ACCESS) disabled
- ✅ Anomalies automatically detected and logged
- ✅ Old logs automatically cleaned up

### Migration

```bash
# Apply database migration
psql -U user -d database -f migrations/20240103_add_activity_log_smart_fields.sql

# Restart application
# That's it!
```

## Event Categories

| Category | Retention | Examples | Default |
|----------|-----------|----------|---------|
| **Critical** | 365 days | LOGIN, PASSWORD_CHANGE, 2FA_ENABLE | Always logged |
| **Important** | 180 days | EMAIL_VERIFY, SOCIAL_LOGIN | Always logged |
| **Informational** | 90 days | TOKEN_REFRESH, PROFILE_ACCESS | Anomaly only |

## Common Configurations

### High Security
```bash
LOG_TOKEN_REFRESH=true
LOG_SAMPLE_TOKEN_REFRESH=0.1
LOG_RETENTION_CRITICAL=730
```

### High Performance
```bash
LOG_DISABLED_EVENTS=TOKEN_REFRESH,PROFILE_ACCESS,EMAIL_VERIFY
LOG_RETENTION_CRITICAL=180
LOG_CLEANUP_INTERVAL=12h
```

### Development
```bash
LOG_TOKEN_REFRESH=true
LOG_PROFILE_ACCESS=true
LOG_CLEANUP_ENABLED=false
```

### GDPR Minimal
```bash
LOG_DISABLED_EVENTS=TOKEN_REFRESH,PROFILE_ACCESS
LOG_ANOMALY_DETECTION_ENABLED=false
LOG_RETENTION_CRITICAL=90
```

## Key Environment Variables

| Variable | Default | Purpose |
|----------|---------|---------|
| `LOG_CLEANUP_ENABLED` | `true` | Enable automatic cleanup |
| `LOG_CLEANUP_INTERVAL` | `24h` | How often cleanup runs |
| `LOG_ANOMALY_DETECTION_ENABLED` | `true` | Enable anomaly detection |
| `LOG_ANOMALY_NEW_IP` | `true` | Log on new IP address |
| `LOG_RETENTION_CRITICAL` | `365` | Days to keep critical logs |
| `LOG_RETENTION_IMPORTANT` | `180` | Days to keep important logs |
| `LOG_RETENTION_INFORMATIONAL` | `90` | Days to keep informational logs |
| `LOG_TOKEN_REFRESH` | `false` | Log token refresh events |
| `LOG_PROFILE_ACCESS` | `false` | Log profile access events |

## Anomaly Detection

Automatically logs when:
- ✅ User accesses from new IP address
- ✅ User uses new device/browser
- ⚙️ Optional: Unusual time access
- ⚙️ Optional: Geographic location change

## Database Schema Changes

New fields added to `activity_logs`:
- `severity` (VARCHAR) - CRITICAL, IMPORTANT, INFORMATIONAL
- `expires_at` (TIMESTAMP) - Automatic expiration date
- `is_anomaly` (BOOLEAN) - True if logged due to anomaly

## Quick Checks

### Verify Migration Applied
```sql
\d activity_logs
-- Should show severity, expires_at, is_anomaly columns
```

### Check Configuration Working
```sql
-- Count by severity
SELECT severity, COUNT(*) FROM activity_logs GROUP BY severity;

-- Check expirations set
SELECT severity, MIN(expires_at), MAX(expires_at) 
FROM activity_logs GROUP BY severity;

-- Check anomaly detection
SELECT COUNT(*) FROM activity_logs WHERE is_anomaly = true;
```

### Monitor Cleanup
```sql
-- Logs ready for cleanup
SELECT COUNT(*) FROM activity_logs WHERE expires_at < NOW();

-- Should decrease after cleanup runs
```

## Troubleshooting

### Database still growing fast?
1. Check `LOG_TOKEN_REFRESH=false` and `LOG_PROFILE_ACCESS=false`
2. Verify cleanup enabled: `LOG_CLEANUP_ENABLED=true`
3. Check retention periods not too long

### Not enough audit data?
1. Enable specific events: `LOG_TOKEN_REFRESH=true`
2. Use sampling: `LOG_SAMPLE_TOKEN_REFRESH=0.01` (1%)
3. Verify anomaly detection enabled

### Cleanup not working?
1. Check application logs for errors
2. Verify database permissions
3. Try manual cleanup:
   ```sql
   DELETE FROM activity_logs WHERE expires_at < NOW();
   ```

## Expected Results

### Before
- TOKEN_REFRESH: ~2M logs/month for 1000 users
- PROFILE_ACCESS: ~500K logs/month
- All logs kept forever
- Database: Rapid growth

### After  
- TOKEN_REFRESH: Only anomalies (~100-1000/month)
- PROFILE_ACCESS: Only anomalies (~50-500/month)
- Critical events: Full logging
- Automatic cleanup: Regular size
- Database: 80-95% reduction

## Documentation

- **Full Guide**: `docs/ACTIVITY_LOGGING_GUIDE.md`
- **Environment Vars**: `docs/ENV_VARIABLES.md`
- **Implementation**: `docs/SMART_LOGGING_IMPLEMENTATION.md`
- **Migration**: `migrations/README_SMART_LOGGING.md`
- **API Docs**: `docs/API.md`

## Support

Configuration not working?
1. Check application startup logs
2. Verify environment variables loaded
3. Review `docs/ACTIVITY_LOGGING_GUIDE.md`
4. Check code: `internal/log/` and `internal/config/logging.go`

---

**Remember**: The system is designed to work with zero configuration. Only customize if you have specific requirements!

