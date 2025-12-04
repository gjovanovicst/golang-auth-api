# Quick Setup: Activity Logging Configuration

## Add to Your .env File

Copy and paste these lines into your `.env` file. All are optional - the system works great with defaults!

```bash
# ============================================================================
# Activity Logging Configuration (Optional)
# ============================================================================

# Basic settings (recommended defaults)
LOG_CLEANUP_ENABLED=true
LOG_CLEANUP_INTERVAL=24h
LOG_ANOMALY_DETECTION_ENABLED=true
LOG_RETENTION_CRITICAL=365
LOG_RETENTION_IMPORTANT=180
LOG_RETENTION_INFORMATIONAL=90

# Advanced settings (uncomment to customize)
# LOG_TOKEN_REFRESH=false
# LOG_PROFILE_ACCESS=false
# LOG_SAMPLE_TOKEN_REFRESH=0.01
# LOG_ANOMALY_NEW_IP=true
# LOG_ANOMALY_NEW_USER_AGENT=true
# LOG_CLEANUP_BATCH_SIZE=1000
```

## Presets

### 🔒 High Security

```bash
LOG_TOKEN_REFRESH=true
LOG_PROFILE_ACCESS=true
LOG_SAMPLE_TOKEN_REFRESH=0.1
LOG_SAMPLE_PROFILE_ACCESS=0.1
LOG_RETENTION_CRITICAL=730
LOG_RETENTION_IMPORTANT=365
LOG_RETENTION_INFORMATIONAL=180
```

### ⚡ High Performance

```bash
LOG_DISABLED_EVENTS=TOKEN_REFRESH,PROFILE_ACCESS,EMAIL_VERIFY
LOG_RETENTION_CRITICAL=180
LOG_RETENTION_IMPORTANT=90
LOG_RETENTION_INFORMATIONAL=30
LOG_CLEANUP_INTERVAL=12h
```

### 🧪 Development

```bash
LOG_TOKEN_REFRESH=true
LOG_PROFILE_ACCESS=true
LOG_CLEANUP_ENABLED=false
LOG_RETENTION_CRITICAL=7
```

### 🔐 GDPR Minimal

```bash
LOG_DISABLED_EVENTS=TOKEN_REFRESH,PROFILE_ACCESS
LOG_ANOMALY_DETECTION_ENABLED=false
LOG_RETENTION_CRITICAL=90
LOG_RETENTION_IMPORTANT=60
LOG_RETENTION_INFORMATIONAL=30
```

## No Configuration Needed!

The system works perfectly without any environment variables:
- ✅ High-frequency events (TOKEN_REFRESH, PROFILE_ACCESS) disabled
- ✅ Anomaly detection enabled
- ✅ Automatic cleanup runs daily
- ✅ Smart retention policies applied

## See Also

- [Complete Guide](ACTIVITY_LOGGING_GUIDE.md)
- [All Variables](ENV_VARIABLES.md)
- [Quick Reference](SMART_LOGGING_QUICK_REFERENCE.md)

