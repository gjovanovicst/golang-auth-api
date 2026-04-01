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

## WebAuthn / Passkeys

```bash
WEBAUTHN_RP_ID=localhost                    # Relying Party ID (your domain)
WEBAUTHN_RP_NAME=Auth API                   # Display name shown in browser prompts
WEBAUTHN_RP_ORIGINS=http://localhost:8080   # Comma-separated allowed origins
```

> **Production example:**
> ```bash
> WEBAUTHN_RP_ID=example.com
> WEBAUTHN_RP_NAME=My App
> WEBAUTHN_RP_ORIGINS=https://example.com,https://app.example.com
> ```

---

## Server

```bash
PORT=8080
GIN_MODE=debug          # Use 'release' for production
ADMIN_URL=http://localhost:8080  # Base URL for admin GUI (used in magic link emails)
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

---

## OIDC Provider

Each application can be enabled as a standards-compliant OpenID Connect issuer. Set `OIDC_ENABLED=true` on the application record (via Admin API or GUI) to activate the OIDC endpoints for that app.

```bash
# Base URL used in OIDC issuer and discovery document
PUBLIC_URL=https://auth.example.com

# Token TTL overrides (optional — defaults to global JWT settings)
OIDC_ID_TOKEN_EXPIRATION_MINUTES=60
OIDC_AUTH_CODE_EXPIRATION_MINUTES=5
```

RSA key pairs for RS256 ID token signing are generated automatically per-application and stored in the database. No manual key management is required.

---

## GeoIP / IP Access Rules

IP-based access rules (CIDR blocks, country allow/block lists) require a MaxMind GeoLite2 database file.

```bash
# Path to the MaxMind GeoLite2-City or GeoLite2-Country .mmdb file
GEOIP_DB_PATH=/data/GeoLite2-City.mmdb
```

If `GEOIP_DB_PATH` is not set or the file does not exist, GeoIP lookups are skipped and country-based rules are ignored. CIDR rules continue to work without GeoIP.

---

## SMS / Twilio

SMS-based 2FA requires a Twilio account.

```bash
SMS_PROVIDER=twilio               # Set to 'twilio' to enable SMS sending
SMS_TWILIO_ACCOUNT_SID=ACxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
SMS_TWILIO_AUTH_TOKEN=your_auth_token
SMS_TWILIO_FROM_NUMBER=+15551234567   # Your Twilio phone number
```

If `SMS_PROVIDER` is empty or not set, SMS sending is disabled and the SMS 2FA option is unavailable.

---

## Session Groups

Session groups link multiple applications so that a single login is valid across all apps in the group. The expiry detection service watches for Redis TTL expirations and revokes cross-app sessions automatically when `GlobalLogout=true` on the group.

```bash
# Enable Redis keyspace notifications for real-time session expiry detection.
# Set to "Ex" for expired key events. The Docker Compose Redis service is
# already configured with this value.
REDIS_NOTIFY_KEYSPACE_EVENTS=Ex

# Enable expiry-triggered group-wide session revocation (default: true).
# Set to false to disable automatic cross-app revocation on session expiry
# while keeping manual (logout-triggered) group revocation active.
SESSION_GROUP_EXPIRY_REVOCATION_ENABLED=true

# Fallback periodic scan interval used when keyspace notifications are not
# available. Accepts Go duration strings: 5m, 10m, 1h, etc.
SESSION_GROUP_EXPIRY_SCAN_INTERVAL=5m

# Enable the keyspace notification listener (default: true when
# REDIS_NOTIFY_KEYSPACE_EVENTS is set). Set to false to rely on the
# periodic fallback scanner only.
SESSION_GROUP_KEYSYSPACE_NOTIF_ENABLED=true
```

> **Redis requirement:** Real-time expiry detection requires the Redis server to be started with `--notify-keyspace-events Ex`. The bundled `docker-compose.yml` and `docker-compose.dev.yml` already include this flag. For externally managed Redis, add `notify-keyspace-events Ex` to your `redis.conf`.

For architecture details and testing scenarios, see [Session Group Expiry Detection](session-group-expiry.md).
