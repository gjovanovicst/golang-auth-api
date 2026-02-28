# Activity Logging

A smart activity logging system that balances security auditing with database performance. Uses intelligent categorization, anomaly detection, and automatic cleanup to reduce database bloat by 80-95% while maintaining critical security data.

For the complete implementation details, see [Activity Logging Guide](features/ACTIVITY_LOGGING_GUIDE.md).

---

## Event Categories

| Severity | Events | Retention | Always Logged |
|----------|--------|-----------|---------------|
| **Critical** | LOGIN, LOGOUT, PASSWORD_CHANGE, 2FA_ENABLE/DISABLE | 1 year | Yes |
| **Important** | REGISTER, EMAIL_VERIFY, SOCIAL_LOGIN, PROFILE_UPDATE | 6 months | Yes |
| **Informational** | TOKEN_REFRESH, PROFILE_ACCESS | 3 months | Only on anomalies |

---

## Anomaly Detection

Informational events are automatically logged when unusual activity is detected:

- New IP address detected
- New device or browser (user agent) detected
- Configurable analysis window (default: 30 days)

---

## Default Behavior

- All critical and important security events are logged
- Token refreshes and profile access are **not** logged by default (high frequency)
- Both are logged if an anomaly is detected (new IP or device)
- Automatic cleanup runs based on retention policies

---

## Configuration

```bash
# High-frequency events (default: disabled)
LOG_TOKEN_REFRESH=false
LOG_PROFILE_ACCESS=false

# Anomaly detection (default: enabled)
LOG_ANOMALY_DETECTION_ENABLED=true
LOG_ANOMALY_NEW_IP=true
LOG_ANOMALY_NEW_USER_AGENT=true

# Retention policies (days)
LOG_RETENTION_CRITICAL=365      # 1 year
LOG_RETENTION_IMPORTANT=180     # 6 months
LOG_RETENTION_INFORMATIONAL=90  # 3 months

# Automatic cleanup
LOG_CLEANUP_ENABLED=true
LOG_CLEANUP_INTERVAL=24h
```

---

## Related Documentation

- [Activity Logging Guide](features/ACTIVITY_LOGGING_GUIDE.md) - Complete guide
- [Quick Setup](features/QUICK_SETUP_LOGGING.md) - Quick setup instructions
- [Smart Logging Reference](features/SMART_LOGGING_QUICK_REFERENCE.md) - Quick reference card
