# Configuration

All configuration is managed through environment variables. Copy `.env.example` to `.env` and update the values for your environment.

For the complete reference with all available options, see [Environment Variables Reference](guides/ENV_VARIABLES.md).

---

## Database

```bash
DB_HOST=postgres        # Use 'localhost' for local dev without Docker
DB_PORT=5432
DB_USER=your_db_user
DB_PASSWORD=your_db_password
DB_NAME=auth_db
```

---

## Redis

```bash
REDIS_ADDR=redis:6379   # Use 'localhost:6379' for local dev without Docker
REDIS_PASSWORD=         # Optional
REDIS_DB=0
```

---

## JWT

```bash
JWT_SECRET=your-strong-secret-key-here-change-in-production
ACCESS_TOKEN_EXPIRATION_MINUTES=15
REFRESH_TOKEN_EXPIRATION_HOURS=720  # 30 days
```

---

## Email

```bash
EMAIL_HOST=smtp.gmail.com
EMAIL_PORT=587
EMAIL_USERNAME=your_email@gmail.com
EMAIL_PASSWORD=your_app_password
EMAIL_FROM=noreply@yourapp.com
```

---

## Social Authentication

OAuth credentials can be configured in two ways:

1. **Environment variables** - Used for the default application
2. **Database (Admin API)** - Per-application configuration, takes precedence over env vars

### Environment Variables

```bash
# Google OAuth2
GOOGLE_CLIENT_ID=your_google_client_id
GOOGLE_CLIENT_SECRET=your_google_client_secret
GOOGLE_REDIRECT_URL=http://localhost:8080/auth/google/callback

# Facebook OAuth2
FACEBOOK_CLIENT_ID=your_facebook_app_id
FACEBOOK_CLIENT_SECRET=your_facebook_app_secret
FACEBOOK_REDIRECT_URL=http://localhost:8080/auth/facebook/callback

# GitHub OAuth2
GITHUB_CLIENT_ID=your_github_client_id
GITHUB_CLIENT_SECRET=your_github_client_secret
GITHUB_REDIRECT_URL=http://localhost:8080/auth/github/callback
```

### Database Configuration (Recommended for Multi-Tenant)

Migrate existing env var credentials to the database:

```bash
go run cmd/migrate_oauth/main.go
```

Or configure per-application via the Admin API:

```bash
curl -X POST http://localhost:8080/admin/oauth-providers \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <admin-token>" \
  -d '{
    "app_id": "your-app-id",
    "provider": "google",
    "client_id": "your-google-client-id",
    "client_secret": "your-google-client-secret",
    "redirect_url": "https://yourapp.com/auth/google/callback",
    "is_enabled": true
  }'
```

For more details, see the [Multi-App OAuth Config Guide](guides/multi-app-oauth-config.md).

---

## Server

```bash
PORT=8080
GIN_MODE=debug          # Use 'release' for production
```

---

## Activity Logging

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

For the complete logging configuration guide, see [Activity Logging](activity-logging.md).
