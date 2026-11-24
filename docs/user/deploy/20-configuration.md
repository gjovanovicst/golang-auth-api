# Configuration

The API reads configuration from environment variables, with `.env` loaded on startup
(existing environment variables win). Every value can be inspected and many can be
overridden per application from the admin GUI.

## Core

| Variable | Default | Purpose |
|---|---|---|
| `PORT` | `8080` | HTTP listen port |
| `ADMIN_API_KEY` | — | Key for `X-Admin-API-Key` (`/admin/*`) |
| `ADMIN_SESSION_EXPIRATION_HOURS` | `8` | Admin GUI session lifetime |
| `ADMIN_EMAIL` | — | Recipient for system notifications (API key expiry) |
| `ADMIN_URL` | — | Base URL used in admin-facing links |
| `FRONTEND_URL` | `http://localhost:5173` | Default frontend used in emails |
| `PUBLIC_URL` | `http://localhost:8080` | Public base URL (OIDC issuer, links) |

## Database and Redis

| Variable | Default | Purpose |
|---|---|---|
| `DB_HOST` / `DB_PORT` | — / `5432` | PostgreSQL |
| `DB_USER` / `DB_PASSWORD` / `DB_NAME` | — | Credentials and database |
| `REDIS_ADDR` | — | `host:port` |
| `REDIS_PASSWORD` | — | Optional |
| `REDIS_DB` | `0` | Logical database |
| `REDIS_NOTIFY_KEYSPACE_EVENTS` | — | Set `Ex` for session-group expiry events |

The API **fails fast** if PostgreSQL or Redis is unreachable: a broken dependency is not
allowed to serve half-working authentication.

## Tokens

| Variable | Default | Purpose |
|---|---|---|
| `JWT_SECRET` | — | HMAC key; use at least 32 random characters |
| `ACCESS_TOKEN_EXPIRATION_MINUTES` | `15` | Access token lifetime |
| `REFRESH_TOKEN_EXPIRATION_HOURS` | `720` | Refresh token lifetime |
| `OAUTH_STATE_SECRET` | `JWT_SECRET` | HMAC key for OAuth `state` (≥ 32 bytes) |
| `OAUTH_STATE_COOKIE_BINDING` | `true` | Bind the state to the initiating browser |
| `OAUTH_STATE_COOKIE_SAMESITE` | `none` | `none` for cross-site frontends, `lax`/`strict` for same-site |

Per-application overrides: `access_token_ttl_minutes` and `refresh_token_ttl_hours`
(0 = use the global value).

## Social providers

```
GOOGLE_CLIENT_ID / GOOGLE_CLIENT_SECRET / GOOGLE_REDIRECT_URL
FACEBOOK_CLIENT_ID / FACEBOOK_CLIENT_SECRET / FACEBOOK_REDIRECT_URL
GITHUB_CLIENT_ID / GITHUB_CLIENT_SECRET / GITHUB_REDIRECT_URL
```

Credentials can also be stored per application in the GUI (*Applications → OAuth*), which
is required for multi-tenant deployments.

## OIDC provider

| Variable | Default | Purpose |
|---|---|---|
| `OIDC_ENABLED` | `false` | Master switch for `/oidc/*` |
| `OIDC_ID_TOKEN_EXPIRATION_MINUTES` | `60` | ID token lifetime |
| `OIDC_AUTH_CODE_EXPIRATION_MINUTES` | `10` | Authorization code lifetime |
| `OIDC_DEFAULT_APP_ID` | `0000…0001` | Target of `/.well-known/openid-configuration` |

## WebAuthn

```
WEBAUTHN_RP_ID=localhost            # registrable domain, no scheme/port
WEBAUTHN_RP_NAME="Auth API"
WEBAUTHN_RP_ORIGINS=http://localhost:8080
```

`RP_ID` must match the domain users see in the browser; origins must list every frontend
origin allowed to create passkeys. Wrong values produce `SecurityError` in the browser.

## SMS

```
SMS_PROVIDER=twilio
SMS_TWILIO_ACCOUNT_SID=AC…
SMS_TWILIO_AUTH_TOKEN=…
SMS_TWILIO_FROM_NUMBER=+1555…
```

Without a provider SMS is disabled and SMS 2FA / SMS recovery are unavailable. The same
credentials power both.

## CORS and trusted-device cookies

| Variable | Default |
|---|---|
| `CORS_ALLOWED_ORIGINS` | `http://localhost:3000,http://localhost:5173,http://localhost:5174,http://localhost:5175,http://localhost:8080` |
| `CORS_ALLOWED_METHODS` | `GET,POST,PUT,DELETE,OPTIONS,HEAD` |
| `CORS_ALLOWED_HEADERS` | `Origin,Content-Type,…` (includes `Authorization`, `X-App-ID`) |
| `CORS_ALLOW_CREDENTIALS` | `true` |
| `TRUSTED_DEVICE_COOKIE_SAMESITE` | `none` (use `lax` for same-site deployments) |

CORS is also editable from the GUI (*Settings → CORS*).

## Session groups and GeoIP

| Variable | Default | Purpose |
|---|---|---|
| `SESSION_GROUP_EXPIRY_REVOCATION_ENABLED` | `true` | Revoke peer sessions on expiry |
| `SESSION_GROUP_EXPIRY_SCAN_INTERVAL` | `5m` | Fallback scan interval |
| `SESSION_GROUP_KEYSYSPACE_NOTIF_ENABLED` | `true` | Listen to Redis expiry events |
| `GEOIP_DB_PATH` | — | MaxMind database for IP rules / anomaly detection |

## Precedence, again

```
environment  →  application (database)  →  built-in default
```

An environment variable always wins over the GUI. If a setting "does not stick", check the
environment before filing a bug — the GUI shows the effective value in *Settings → Info*.
