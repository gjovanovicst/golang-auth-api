# Environment Variables Reference

This document lists all environment variables used by the Authentication API.

## Quick Copy-Paste for .env File

Add these optional activity logging variables to your `.env` file:

```bash
# Activity Logging (Optional - defaults work great!)
LOG_CLEANUP_ENABLED=true
LOG_CLEANUP_INTERVAL=24h
LOG_ANOMALY_DETECTION_ENABLED=true
LOG_RETENTION_CRITICAL=365
LOG_RETENTION_IMPORTANT=180
LOG_RETENTION_INFORMATIONAL=90
```

## Database Configuration

```bash
# PostgreSQL connection
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=your_password
DB_NAME=auth_db
DB_SSL_MODE=disable
```

## Redis Configuration

```bash
# Redis connection
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=
REDIS_DB=0
```

## JWT Configuration

```bash
# JWT secrets (REQUIRED - use strong random values in production)
JWT_SECRET=your-super-secret-jwt-key-change-this-in-production
REFRESH_TOKEN_SECRET=your-super-secret-refresh-token-key-change-this

# Token expiration
ACCESS_TOKEN_EXPIRATION_MINUTES=15
REFRESH_TOKEN_EXPIRATION_HOURS=720  # 30 days
```

## Email Configuration

```bash
# SMTP settings for email verification
SMTP_HOST=smtp.gmail.com
SMTP_PORT=587
SMTP_USERNAME=your-email@gmail.com
SMTP_PASSWORD=your-app-password
SMTP_FROM=noreply@yourapp.com
```

## Social Authentication (OAuth2)

### Google OAuth

```bash
GOOGLE_CLIENT_ID=your-google-client-id.apps.googleusercontent.com
GOOGLE_CLIENT_SECRET=your-google-client-secret
GOOGLE_REDIRECT_URL=http://localhost:8080/auth/google/callback
```

### Facebook OAuth

```bash
FACEBOOK_APP_ID=your-facebook-app-id
FACEBOOK_APP_SECRET=your-facebook-app-secret
FACEBOOK_REDIRECT_URL=http://localhost:8080/auth/facebook/callback
```

### GitHub OAuth

```bash
GITHUB_CLIENT_ID=your-github-client-id
GITHUB_CLIENT_SECRET=your-github-client-secret
GITHUB_REDIRECT_URL=http://localhost:8080/auth/github/callback
```

## Application Settings

```bash
# Server port
PORT=8080

# Application environment
APP_ENV=development  # development, staging, production

# Frontend URL (for CORS and redirects)
FRONTEND_URL=http://localhost:3000
```

## Activity Logging Configuration

### Basic Event Control

```bash
# Disable specific events (comma-separated list)
# Default: TOKEN_REFRESH and PROFILE_ACCESS are disabled
LOG_DISABLED_EVENTS=TOKEN_REFRESH,PROFILE_ACCESS

# Enable specific informational events
LOG_TOKEN_REFRESH=false              # Enable token refresh logging (default: false)
LOG_PROFILE_ACCESS=false             # Enable profile access logging (default: false)
```

### Sampling Configuration

```bash
# Sampling rates for high-frequency events (0.0 to 1.0)
# Only applies when the event is enabled
LOG_SAMPLE_TOKEN_REFRESH=0.01        # Log 1% of token refreshes (default: 0.01)
LOG_SAMPLE_PROFILE_ACCESS=0.01       # Log 1% of profile accesses (default: 0.01)
```

### Anomaly Detection

```bash
# Master switch for anomaly detection
LOG_ANOMALY_DETECTION_ENABLED=true   # Enable anomaly-based logging (default: true)

# Specific anomaly triggers
LOG_ANOMALY_NEW_IP=true              # Log when new IP detected (default: true)
LOG_ANOMALY_NEW_USER_AGENT=true      # Log when new user agent detected (default: true)
LOG_ANOMALY_GEO_CHANGE=false         # Log on geographic change (default: false, requires GeoIP)
LOG_ANOMALY_UNUSUAL_TIME=false       # Log unusual time access (default: false)

# Pattern analysis window
LOG_ANOMALY_SESSION_WINDOW=720h      # 30 days - how long to remember patterns (default: 720h)
```

### Retention Policies

```bash
# Retention periods in days by severity level
LOG_RETENTION_CRITICAL=365           # Critical events: 1 year (default: 365)
LOG_RETENTION_IMPORTANT=180          # Important events: 6 months (default: 180)
LOG_RETENTION_INFORMATIONAL=90       # Informational events: 3 months (default: 90)
```

### Cleanup Service

```bash
# Automatic cleanup configuration
LOG_CLEANUP_ENABLED=true             # Enable automatic cleanup (default: true)
LOG_CLEANUP_INTERVAL=24h             # Cleanup frequency (default: 24h)
LOG_CLEANUP_BATCH_SIZE=1000          # Logs per batch (default: 1000)
LOG_ARCHIVE_BEFORE_CLEANUP=false     # Archive to file before delete (default: false)
```

## Security Settings

```bash
# Rate limiting (future feature)
RATE_LIMIT_ENABLED=true
RATE_LIMIT_MAX_REQUESTS=100
RATE_LIMIT_WINDOW=60s

# CORS allowed origins (comma-separated)
CORS_ALLOWED_ORIGINS=http://localhost:3000,https://yourapp.com
```

## Quick Configuration Presets

### Development Environment

```bash
# Minimal logging, no cleanup
LOG_CLEANUP_ENABLED=false
LOG_TOKEN_REFRESH=true
LOG_PROFILE_ACCESS=true
LOG_SAMPLE_TOKEN_REFRESH=1.0
LOG_SAMPLE_PROFILE_ACCESS=1.0
LOG_RETENTION_CRITICAL=7
LOG_RETENTION_IMPORTANT=7
LOG_RETENTION_INFORMATIONAL=7
```

### Production - High Security

```bash
# Log everything, long retention
LOG_TOKEN_REFRESH=true
LOG_PROFILE_ACCESS=true
LOG_SAMPLE_TOKEN_REFRESH=0.1         # 10% sampling
LOG_SAMPLE_PROFILE_ACCESS=0.1
LOG_RETENTION_CRITICAL=730           # 2 years
LOG_RETENTION_IMPORTANT=365          # 1 year
LOG_RETENTION_INFORMATIONAL=180      # 6 months
LOG_CLEANUP_ENABLED=true
LOG_CLEANUP_INTERVAL=24h
```

### Production - High Performance

```bash
# Minimal logging, aggressive cleanup
LOG_DISABLED_EVENTS=TOKEN_REFRESH,PROFILE_ACCESS,EMAIL_VERIFY
LOG_RETENTION_CRITICAL=180           # 6 months
LOG_RETENTION_IMPORTANT=90           # 3 months
LOG_RETENTION_INFORMATIONAL=30       # 1 month
LOG_CLEANUP_ENABLED=true
LOG_CLEANUP_INTERVAL=12h
LOG_CLEANUP_BATCH_SIZE=5000
```

### GDPR Compliant

```bash
# Minimal tracking, short retention
LOG_DISABLED_EVENTS=TOKEN_REFRESH,PROFILE_ACCESS
LOG_ANOMALY_DETECTION_ENABLED=false
LOG_RETENTION_CRITICAL=90
LOG_RETENTION_IMPORTANT=60
LOG_RETENTION_INFORMATIONAL=30
LOG_CLEANUP_ENABLED=true
LOG_CLEANUP_INTERVAL=24h
```

## Notes

- All time durations use Go duration format: `24h`, `30m`, `720h`, etc.
- Sampling rates are decimals: `0.01` = 1%, `0.5` = 50%, `1.0` = 100%
- Boolean values: `true`, `false`, `1`, `0`, `yes`, `no`
- Lists are comma-separated with no spaces
- Values with spaces should be quoted if set in shell

## Validation

After setting environment variables, verify they're loaded:

```bash
# In the application logs, you should see:
# "Activity log cleanup service initialized (interval: 24h)"
# This confirms the configuration is loaded correctly
```

## Security Warnings

⚠️ **NEVER** commit `.env` files to version control
⚠️ Use strong, random secrets for JWT keys in production
⚠️ Use environment-specific configuration management in production
⚠️ Rotate secrets regularly
⚠️ Review and audit logging configuration quarterly

## See Also

- [Activity Logging Guide](ACTIVITY_LOGGING_GUIDE.md) - Detailed logging configuration
- [API Documentation](API.md) - API endpoints and event types
- [Migration Guide](../migrations/README_SMART_LOGGING.md) - Database migration instructions

