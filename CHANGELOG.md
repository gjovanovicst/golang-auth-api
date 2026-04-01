# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

> **Pre-release note:** All versions below are pre-release development milestones
> (`1.0.0-alpha.N`). The first official public release will be `1.0.0`.

## [Unreleased]

_Future enhancements planned for the `1.0.0` official release._

### Planned — Application-Level Customization
- Custom login page branding per app (logo, colors)
- Configurable password policies per app (min length, complexity, history)
- Configurable token TTLs per application (currently global)

### Planned — Multi-Factor Recovery
- SMS-based recovery (even if not primary 2FA method)

---

## [1.0.0-alpha.7] - 2026-04-01

### Added

#### Session Groups
- **Cross-application session groups** — group multiple applications under a named session group so that a single authentication event is valid across all apps in the group (similar to Google's cross-product SSO)
- New models: `SessionGroup` and `SessionGroupApp` (`pkg/models/session_group.go`); a session group belongs to a tenant, an application can belong to at most one group
- **`GlobalLogout` flag** — when `true`, logging out of (or expiry of a session in) any app in the group immediately revokes the user's sessions in all other apps of the group
- New package: `internal/sessiongroup/` with two components:
  - `revoke.go` — shared group-wide session revocation utility (used by both logout and expiry detection)
  - `expiry.go` — real-time session expiry detection via Redis keyspace notifications (`REDIS_NOTIFY_KEYSPACE_EVENTS=Ex`) with a configurable periodic fallback scanner
- Admin API CRUD endpoints for session group management
- Admin GUI **Session Groups** page (create, edit, delete, manage member apps)
- New env vars for expiry detection (see Configuration section below)
- Docker Compose `redis` service updated with `--notify-keyspace-events Ex` for out-of-the-box real-time expiry support
- New documentation: `docs/session-group-expiry.md`

#### New environment variables — Session Groups

| Variable | Default | Description |
|---|---|---|
| `REDIS_NOTIFY_KEYSPACE_EVENTS` | _(unset)_ | Set to `Ex` to enable Redis expired-key events for real-time expiry detection |
| `SESSION_GROUP_EXPIRY_REVOCATION_ENABLED` | `true` | Enable/disable expiry-triggered group-wide session revocation |
| `SESSION_GROUP_EXPIRY_SCAN_INTERVAL` | `5m` | Fallback periodic scan interval when keyspace notifications are not available |
| `SESSION_GROUP_KEYSYSPACE_NOTIF_ENABLED` | `true` | Enable/disable the keyspace notification listener |

#### CodeMirror Email Template Editor
- **Rich HTML editor** in the Admin GUI email template editor, powered by CodeMirror 6
- Syntax highlighting, line numbers, and bracket matching for HTML email templates
- **Template variable hinting** — inline autocomplete for all available email template variables (e.g. `{{.VerifyURL}}`, `{{.UserName}}`)
- **Popup editor window** — a dedicated full-screen editor window (`email_template_editor_window.tmpl`) for distraction-free editing
- Download helper scripts for CodeMirror assets:
  - `scripts/download-codemirror-assets.sh` (Linux/macOS)
  - `scripts/download-codemirror-assets.bat` (Windows)

#### OIDC Cross-App Session Revocation on Logout
- `POST /oidc/:app_id/end_session` now triggers group-wide session revocation when the application belongs to a session group with `GlobalLogout=true`
- Consistent behavior between standard logout (`POST /logout`) and OIDC RP-initiated logout

#### CORS Configuration via Admin GUI
- CORS allowed origins are now configurable through the Admin GUI **Settings** page, eliminating the need to restart the server for origin changes
- Settings are resolved with the existing 3-tier precedence: environment variable → database → built-in default

#### Social Users: Set Initial Password
- Social-only users (registered exclusively via OAuth2) can now set a local password via `PUT /profile/password` without providing a current password (since none exists)
- Enables hybrid authentication for users who originally signed up through social login

#### Enhanced Email Verification for Social Login Re-registration
- When a user attempts to re-register with an email already associated with a social account but not yet verified, the pending email verification is now re-triggered automatically
- Improves UX for users who signed up via social login and later try the email/password flow

#### Admin GUI: SMS 2FA and Trusted Device Form Data
- SMS 2FA configuration options and trusted device management are now correctly included in Admin GUI form submissions (previously missing from `formData`)

### Fixed

- **Trusted devices list** — the `GET /2fa/trusted-devices` endpoint (and its admin counterpart) now filters out expired trusted devices; only active, non-expired devices are returned
- **Change password: current password optional** — `PUT /profile/password` no longer requires the `current_password` field when the user has no local password set (social-only accounts); fixes a 422 validation error that prevented social users from ever setting a password
- **RBAC member role: settings permissions** — the system `member` role was missing `settings:read` and `settings:write` permissions, causing 403 errors on all 2FA self-service endpoints (TOTP setup, email 2FA, SMS 2FA, backup email, passkeys, trusted devices, phone management); SQL migration `20260317_add_settings_permissions_to_member.sql` grants these permissions to the `member` role in every existing application

### Security

- **CWE-269: API key empty scope privilege escalation (fix, closes #13)** — `HasScope()` in `internal/middleware/scope.go` previously treated a DB-backed API key with an empty scope list as fully permissive, effectively granting admin-level access to any key with no scopes configured. The scope check now **denies by default** when the granted scope list is empty. To grant unrestricted access to a DB-backed key, the key must explicitly include the `"*"` scope. The static `ADMIN_API_KEY` environment variable remains unconditionally permissive and is unaffected by this change.

  > **Action required for existing installations:** Any DB-backed API key that was intentionally created without scopes to obtain full access must be updated — add the `"*"` scope or the specific scopes it needs via the Admin GUI or API.

  Credit: **tinokyo** ([@Tinocio](https://github.com/Tinocio)) — thank you for the detailed responsible disclosure including root cause analysis, proof of concept, and suggested fix.

### Style

- Dark mode table row styles improved for better visibility across all Admin GUI pages

---

## [1.0.0-alpha.6] - 2026-03-15

### Added

#### OpenID Connect (OIDC) Provider
- **Full OIDC provider** — the API can now act as a standards-compliant OIDC identity provider (opt-in via `OIDC_ENABLED=true`)
- Discovery document: `GET /.well-known/openid-configuration` (global redirect) and `GET /oidc/:app_id/.well-known/openid-configuration`
- JWKS endpoint: `GET /oidc/:app_id/.well-known/jwks.json`
- Authorization endpoint with login/consent UI: `GET /oidc/:app_id/authorize`, `POST /oidc/:app_id/authorize`
- Token endpoint (authorization_code, client_credentials, refresh_token grants): `POST /oidc/:app_id/token`
- UserInfo endpoint: `GET|POST /oidc/:app_id/userinfo`
- Token introspection: `POST /oidc/:app_id/introspect`
- Token revocation: `POST /oidc/:app_id/revoke`
- End session (RP-initiated logout): `GET|POST /oidc/:app_id/end_session`
- Admin API for OIDC client management: `POST|GET|PUT|DELETE /admin/oidc/apps/:id/clients`, plus secret rotation
- Admin GUI pages for managing OIDC clients (create, edit, delete, rotate secret)
- New models: `OIDCClient` (`pkg/models/oidc_client.go`), `OIDCAuthCode` (`pkg/models/oidc_auth_code.go`)
- New DTOs: `pkg/dto/oidc.go`
- New domain package: `internal/oidc/` (handler, service, repository, id\_token, keys)
- New env vars: `OIDC_ENABLED`, `PUBLIC_URL`, `OIDC_ID_TOKEN_EXPIRATION_MINUTES`, `OIDC_AUTH_CODE_EXPIRATION_MINUTES`
- SQL migration: `20260306_add_oidc.sql`
- Supports scopes: `openid`, `profile`, `email`, `roles`, `offline_access`; PKCE (S256); RS256 signed ID tokens

#### Webhook System
- **Event-driven webhooks** — applications can register HTTPS endpoints to receive real-time event notifications
- Events delivered: `user.registered`, `user.verified`, `user.login`, `user.password_changed`, `2fa.enabled`, `2fa.disabled`, `social.linked`, `social.unlinked`
- Delivery queue with async background dispatcher, automatic retries, and per-delivery status tracking
- Admin API: `GET|POST /admin/webhooks`, `GET|POST /admin/webhooks/apps/:app_id`, `PUT /admin/webhooks/:id/toggle`, `DELETE /admin/webhooks/:id`, plus delivery history endpoints
- App-scoped API: `GET|POST|PUT|DELETE /app/:id/webhooks`, `GET /app/:id/webhooks/deliveries`
- Admin GUI Webhooks page (create, toggle, delete, view delivery history)
- New models: `WebhookEndpoint` (`pkg/models/webhook_endpoint.go`), `WebhookDelivery` (`pkg/models/webhook_delivery.go`)
- New DTOs: `pkg/dto/webhook.go`
- New domain package: `internal/webhook/` (handler, service, repository)
- SQL migration: `20260305_add_webhooks.sql`

#### Brute-Force Protection
- **Account lockout** — after configurable failed login attempts, accounts are temporarily locked with progressive backoff
- **CAPTCHA trigger** — after N failures, a CAPTCHA challenge is required before the next attempt
- Admin GUI: unlock button on User detail page (`PUT /gui/users/:id/unlock`)
- Admin API: configurable per-application settings via `20260305_add_app_bruteforce_settings.sql` migration
- New package: `internal/bruteforce/` (service, captcha)
- Login failure reasons tracked in Prometheus metrics (`auth_login_failure_total` with `reason` label)

#### GeoIP Service and IP Access Rules
- **IP-based access control** — allow or block login attempts by CIDR range or country code, per application
- Uses MaxMind GeoLite2 database for country/city resolution (gracefully disabled when `GEOIP_DB_PATH` is not set)
- Admin API: `GET|POST|PUT|DELETE /admin/apps/:id/ip-rules`, `GET /admin/apps/:id/ip-rules/:rule_id`, `POST /admin/apps/:id/ip-rules/check`
- Admin GUI IP Rules page (create, edit, delete, test access)
- Anomaly detection enhanced: new IP addresses now resolve to city/country and trigger notification emails
- New model: `IPRule` (`pkg/models/ip_rule.go`)
- New DTOs: `pkg/dto/ip_rule.go`
- New package: `internal/geoip/` (service, ip\_rules repository/evaluator)
- New env var: `GEOIP_DB_PATH` (optional — path to MaxMind `.mmdb` file)

#### Health Check and Prometheus Metrics
- `GET /health` — checks PostgreSQL, Redis, and SMTP connectivity with latency; returns 200 when healthy, 503 when any component is down
- `GET /metrics` — Prometheus exposition format; requires Admin API Key
- Metrics tracked: `http_requests_total`, `http_request_duration_seconds`, `auth_login_success_total`, `auth_login_failure_total`, `auth_register_total`, `auth_logout_total`, DB connection pool gauges, `active_sessions_total`
- `PrometheusMiddleware` instruments every HTTP request automatically
- Admin GUI Monitoring page with live health and metrics summary
- New package: `internal/health/` (handler)
- New DTOs: `pkg/dto/health.go`

#### SMS Two-Factor Authentication
- **SMS-based 2FA** via Twilio — users can receive one-time codes by SMS as a second factor
- New endpoints: `POST /2fa/sms/enable` (protected), `POST /2fa/sms/resend` (public, during login)
- Phone number management: `POST /phone` (add), `POST /phone/verify`, `DELETE /phone`, `GET /phone/status`
- New package: `internal/sms/` (sender interface, Twilio implementation)
- New env vars: `SMS_PROVIDER` (`"twilio"` or empty to disable), `SMS_TWILIO_ACCOUNT_SID`, `SMS_TWILIO_AUTH_TOKEN`, `SMS_TWILIO_FROM_NUMBER`
- Graceful degradation: SMS is silently disabled when `SMS_PROVIDER` is not set

#### Backup Email Two-Factor Authentication
- Users can register a secondary email address as a 2FA fallback channel
- New endpoints: `POST /2fa/backup-email` (add), `DELETE /2fa/backup-email` (remove), `GET /2fa/backup-email/status`, `POST /2fa/backup-email/enable`, `POST /2fa/backup-email/disable`, `GET /2fa/backup-email/verify`, `POST /2fa/backup-email/resend`
- Admin GUI My Account: manage backup email for admin accounts
- SQL migration: `20260309_seed_backup_email_verification_type.sql`

#### Trusted Device Management
- **"Remember this device"** — users can mark a device as trusted to skip 2FA for a configurable period
- New endpoints: `GET /2fa/trusted-devices`, `DELETE /2fa/trusted-devices/:id`, `DELETE /2fa/trusted-devices` (revoke all)
- Admin API: `GET|DELETE /admin/users/:id/trusted-devices`, `DELETE /admin/users/:id/trusted-devices/:device_id`
- Admin GUI: trusted device list on User detail page, My Account page for admin self-management
- New model: `TrustedDevice` (`pkg/models/trusted_device.go`)
- New repository: `internal/twofa/trusted_device_repository.go`

#### Activity Log Export
- `GET /activity-logs/export` — export the authenticated user's logs (CSV)
- `GET /admin/activity-logs/export` — export all users' logs (CSV, admin-only)
- `GET /gui/logs/export` — export from admin GUI

#### User Import/Export
- `GET /admin/users/export` — export user list as CSV
- `POST /admin/users/import` — bulk import users from CSV/JSON
- Admin GUI: export button and import modal on Users page
- New DTO: `pkg/dto/user_import.go`
- New helper: `internal/user/import.go`

#### API Key Scopes and Usage Tracking
- API keys now support granular permission **scopes** to limit which endpoints a key can access
- **Usage tracking**: every API key request is recorded in `api_key_usages` table; viewable on a per-key usage page in the Admin GUI
- **Expiry notification service**: background service emails admins when API keys are nearing expiration
- New model: `ApiKeyUsage` (`pkg/models/api_key_usage.go`)
- New service: `internal/admin/apikey_notification.go`
- SQL migrations: `20260305_add_api_key_scopes_columns.sql`, `20260305_create_api_key_usages.sql`, `20260305_seed_api_key_expiring_soon_email_type.sql`

#### Available 2FA Methods Endpoint
- `GET /2fa/methods` — public endpoint returning the 2FA methods enabled for the current application; used by login UI to show available options

#### App-Config Endpoint
- `GET /app-config/:app_id` — returns the public login configuration for an application (enabled auth methods, feature flags); used by frontend login/register UI without authentication

#### Configurable Email Action Link Paths
- Per-application configuration for the path segments used in email action links (verify email, password reset, magic link)
- SQL migration: `20260314_add_app_link_paths.sql`

#### Application-Level 2FA Enforcement
- Applications can now enforce 2FA for all users via a feature flag
- SQL migration: `20260310_add_two_fa_previous_method.sql` — tracks the previous 2FA method when switching methods

#### Application Customization
- New per-application customization fields (frontend URL, branding hints)
- SQL migration: `20260311_add_app_customization.sql`

#### Security Email Types
- Seeded new security-related email types: anomaly notifications (new device login, suspicious activity)
- SQL migrations: `20260305_seed_security_email_types.sql`, `20260305_seed_api_key_expiring_soon_email_type.sql`

#### Anomaly Notification Emails
- When anomaly detection fires, the system now sends email notifications to the affected user
- Two email types: `new_device_login` and `suspicious_activity`
- Wired via callback from log service to email service in `main.go`

#### OIDC Login Activity Logging
- Successful OIDC logins are recorded in the activity log with `OIDC_LOGIN` event type

### Changed

#### 2FA Method Switching
- Previous 2FA method is preserved in `two_fa_previous_method` field when a user switches methods, enabling smoother recovery flows

#### Session Management Enhancement
- Sessions now track a `status` field for richer state management (active, revoked, expired)
- User token blacklist is cleared when a new session starts (prevents stale blacklist entries from blocking valid new sessions)

#### Middleware
- `middleware.Scope()` added for API key scope validation on app-scoped routes
- Rate limits added for OIDC endpoints (`OIDCAuthorizeRateLimit`, `OIDCTokenRateLimit`, `OIDCUserInfoRateLimit`, `OIDCIntrospectRateLimit`, `OIDCRevokeRateLimit`)
- Rate limits added for 2FA SMS and backup-email resend endpoints

#### Admin GUI Expansion
- New pages: **Monitoring** (health check + Prometheus metrics summary), **Webhooks**, **OIDC Clients**, **IP Rules**, **API Key Usage**
- Users page: unlock button for brute-force-locked accounts, trusted device management per user
- My Account page: backup email management, trusted device management
- Dashboard: activity chart updated

### Fixed
- Session: clearing the token blacklist on new session creation prevents a race condition where a fresh token was incorrectly rejected after logout followed by immediate re-login

---

## [1.0.0-alpha.5] - 2026-03-03

### Added

#### WebAuthn / Passkeys
- **Full FIDO2/WebAuthn passkey support** — Registration, two-factor authentication, and passwordless login
- New endpoints: `POST /passkey/register/begin|finish`, `GET /passkeys`, `PUT /passkeys`, `DELETE /passkeys`, `POST /2fa/passkey/begin|finish`, `POST /passkey/login/begin|finish`
- New model: `WebauthnCredential` (`pkg/models/webauthn_credential.go`)
- New DTOs: `pkg/dto/webauthn.go` (passkey list, rename, delete, registration/login options)
- New configuration: `WEBAUTHN_RP_ID`, `WEBAUTHN_RP_NAME`, `WEBAUTHN_RP_ORIGINS` environment variables
- New domain package: `internal/webauthn/` (handler, service, repository, config)

#### Role-Based Access Control (RBAC)
- **Per-application roles, permissions, and user-role assignments** via admin API
- Admin endpoints: `POST/GET /admin/rbac/roles`, `PUT/DELETE /admin/rbac/roles/:id`, `POST/GET /admin/rbac/permissions`, `PUT/DELETE /admin/rbac/permissions/:id`, `POST/GET/DELETE /admin/rbac/user-roles`
- Default system roles seeded on migration: `admin` and `member`
- JWT tokens now include `roles` claim (array of role names per application)
- New models: `Role`, `Permission`, `UserRole` (`pkg/models/role.go`)
- New DTOs: `pkg/dto/rbac.go`
- New domain package: `internal/rbac/` (handler, service, repository)
- SQL migrations: `20260301_add_rbac.sql`, `20260301_seed_rbac_defaults.sql`, `20260302_backfill_member_role.sql`

#### Session Management
- **List and revoke active sessions** for authenticated users
- New endpoints: `GET /sessions` (list active sessions), `DELETE /sessions/:id` (revoke one), `DELETE /sessions` (revoke all)
- New DTOs: `pkg/dto/session.go`
- New domain package: `internal/session/` (handler, service)

#### Magic Link Login
- **Passwordless login via email magic link** for both users and admin accounts
- New endpoints: `POST /magic-link/request`, `POST /magic-link/verify`
- Per-application setting: `magic_link_enabled` (opt-in via Admin API)
- Admin GUI magic link login support
- New environment variable: `ADMIN_URL` (base URL for magic link emails)
- SQL migrations: `20260303_add_admin_magic_link.sql`, `20260303_add_magic_link_settings.sql`, `20260303_seed_magic_link_email_type.sql`

#### Social Account Linking
- **Link and unlink social accounts** to/from existing authenticated users
- New endpoints: `GET /profile/social-accounts`, `DELETE /profile/social-accounts`, `/auth/{provider}/link`, `/auth/{provider}/link/callback` (for Google, Facebook, GitHub)
- Admin GUI social account unlink support on My Account page

#### Resend Email Verification
- New endpoint: `POST /resend-verification` — Resend the email verification link for unverified users

#### Admin GUI Expansion
- **New admin pages**: Roles, Permissions, User Roles, Sessions, My Account
- **My Account page**: Passkey management (register/rename/delete), magic link toggle, 2FA settings, social account unlinking
- **Login page enhancements**: Passkey login option, magic link login option (in addition to password)
- **HTMX sidebar navigation**: Improved admin GUI layout with HTMX-powered sidebar and page containers

#### Admin Email Login
- Admin accounts now support an `email` field and login by email (in addition to username)

### Changed

#### Auth Middleware Hardening
- Auth middleware now validates session existence in Redis on **every authenticated request**
- Revoked or expired sessions are immediately rejected even if the JWT token is still valid
- Improves security by ensuring session revocation takes effect instantly

#### Application Model
- New feature flag fields on `applications` table: `passkey_2fa_enabled`, `passkey_login_enabled`, `magic_link_enabled`, `email_2fa_enabled`, `two_fa_methods`
- All fields managed by GORM AutoMigrate (no manual SQL migration needed)

#### Admin Account Model
- New fields: `email`, `two_fa_enabled`, `two_fa_method`, `two_fa_secret`, `two_fa_recovery_codes`, `magic_link_enabled`

#### JWT Claims
- JWT access tokens now include a `roles` array claim containing the user's role names for the current application

#### New Activity Log Event Types
- Passkey events: `PASSKEY_REGISTER`, `PASSKEY_DELETE`, `PASSKEY_LOGIN`
- Magic link events: `MAGIC_LINK_REQUESTED`, `MAGIC_LINK_LOGIN`, `MAGIC_LINK_FAILED`
- Email events: `EMAIL_VERIFY_RESEND`
- Social linking events: `SOCIAL_ACCOUNT_LINKED`, `SOCIAL_ACCOUNT_UNLINKED`
- All new events default to **Informational** severity with standard retention policies

## [1.0.0-alpha.4] - 2026-02-21

### Added

#### Admin GUI (Stories 1-12)
- **Admin GUI Dashboard** — Full-featured web-based admin panel served at `/gui/*` from the same binary
- **CLI Admin Setup** — `cmd/setup/main.go` interactive wizard for creating the initial admin account with bcrypt-hashed password
- **Session-Based Authentication** — Redis-backed sessions with secure cookies (SameSite=Strict, HttpOnly, Secure)
- **CSRF Protection** — Token-based CSRF middleware for all GUI mutation endpoints
- **Dashboard Page** — Overview with tenant/app/user/log counts and recent activity
- **Tenant Management** — Full CRUD with HTMX single-page interactions, paginated list
- **Application Management** — Full CRUD with tenant filter dropdown, paginated flat list
- **OAuth Config Management** — Full CRUD with provider dropdown, inline enable/disable toggle, secret masking
- **User Management** — Read-only user list with search, inline detail panel, active/inactive toggle with token revocation
- **Activity Log Viewer** — Read-only with multiple filters (event type, severity, app, date range, email search), inline detail panel
- **API Key Management** — Admin-level and per-application API keys with SHA-256 hashed storage, key shown once at creation, revoke/delete support
- **Settings Management** — Accordion-based settings page with lazy-loaded sections, per-setting inline save/reset, registry-based architecture
- **Embedded Static Assets** — Bootstrap 5 CSS/JS, HTMX, Bootstrap Icons all embedded via `go:embed`
- **Go Template Engine** — Custom `gin.HTMLRender` implementation with layout/partial composition

#### New Middleware
- **GUI Auth Middleware** (`internal/middleware/gui_auth.go`) — Session-based authentication for GUI routes
- **CSRF Middleware** (`internal/middleware/csrf.go`) — CSRF token validation for GUI mutations
- **Admin Auth Middleware** (`internal/middleware/admin_auth.go`) — Database-backed API key authentication for `/admin/*` routes
- **App API Key Middleware** (`internal/middleware/app_api_key.go`) — Per-application API key validation (available for future use)
- **Security Headers Middleware** (`internal/middleware/security_headers.go`) — X-Frame-Options, CSP, HSTS, Permissions-Policy, and more
- **Generic Rate Limit Middleware** (`internal/middleware/rate_limit.go`) — Configurable per-route rate limiting with Redis + in-memory fallback

#### New Models & Migrations
- **AdminAccount Model** (`pkg/models/admin_account.go`) — Admin user accounts with bcrypt password hash
- **SystemSetting Model** (`pkg/models/system_setting.go`) — Key-value settings with DB override support
- **ApiKey Model** (`pkg/models/api_key.go`) — Admin and per-app API keys with SHA-256 hash, prefix/suffix display
- **Database Migrations** for admin accounts, system settings, and API keys tables

#### New API Endpoints (GUI)
- `GET /gui/login` — Login page
- `POST /gui/login` — Authenticate admin
- `POST /gui/logout` — End admin session
- `GET /gui/dashboard` — Dashboard with stats
- `GET/POST/PUT/DELETE /gui/tenants/*` — Tenant CRUD
- `GET/POST/PUT/DELETE /gui/apps/*` — Application CRUD
- `GET/POST/PUT/DELETE /gui/oauth/*` — OAuth config CRUD with toggle
- `GET /gui/users/*` — User list, detail, search, toggle active
- `GET /gui/logs/*` — Activity log viewer with filters
- `GET/POST/PUT/DELETE /gui/api-keys/*` — API key management
- `GET/PUT/DELETE /gui/settings/*` — Settings management

### Changed

#### Security Hardening (Story 13)
- **JWT Secret Validation** — `log.Fatalf` if `JWT_SECRET` is empty or less than 32 bytes; lazy initialization via `sync.Once`
- **JWT Token Type Claim** — Added `type` field to JWT claims (`"access"` or `"refresh"`); auth middleware rejects refresh tokens used as access tokens; backward compatible with legacy tokens
- **Password Hashing** — bcrypt cost increased from default (10) to 12 for all password operations
- **CSRF Comparison** — Changed from `==` to `crypto/subtle.ConstantTimeCompare` to prevent timing attacks
- **Cookie Security** — Admin session cookies now use `SameSite=Strict` via `http.SetCookie`
- **CORS Production Safety** — Localhost origins removed from CORS allowlist in release mode; warning logged if `FRONTEND_URL` is empty
- **Password Max Length** — Added `max=128` validation to all 7 password fields across 6 DTOs to prevent bcrypt DoS
- **Error Message Sanitization** — Replaced 6 instances of `err.Error()` leaking internal details in social handler with generic messages
- **Debug Print Removal** — Removed 9 `fmt.Print`/`fmt.Println` debug statements from user, email, and 2FA services
- **SQL Injection Fix** — Fixed `INTERVAL '? days'` (non-parameterized) to `INTERVAL '1 day' * ?` in log cleanup
- **Rate Limiting** — Applied rate limits to 6 public endpoints: `/register` (3/min), `/login` (5/min + lockout), `/refresh-token` (10/min), `/forgot-password` (3/min), `/reset-password` (5/min), `/2fa/login-verify` (5/min + lockout)
- **Security Headers** — Added global middleware: X-Frame-Options (DENY), X-Content-Type-Options (nosniff), Referrer-Policy, CSP (route-aware: strict for API, relaxed for GUI), HSTS (conditional on TLS)

#### Architecture
- **Shared Constants** — Moved session/context keys, cookie helpers, and interfaces to `web/context_keys.go` to resolve import cycles
- **SessionValidator Interface** — `web.SessionValidator` implemented by `AccountService` for middleware decoupling
- **ApiKeyValidator Interface** — `web.ApiKeyValidator` implemented by admin `Repository` for middleware decoupling

#### Testing & Documentation (Story 14)
- **JWT Test Fix** — Replaced `init()` with `sync.Once` lazy initialization to fix test ordering issues
- **New Test Suites** — Rate limiter (17 tests), security headers (6 tests), DTO validation (17 tests), CSRF comparison (10 tests), error types (5 tests), API key utilities (6 tests)
- **Swagger Updates** — Added `@Failure 429` annotations to all 6 rate-limited endpoints; regenerated swagger docs

### Fixed
- **JWT init() ordering** — `init()` in `pkg/jwt/jwt.go` called `log.Fatalf` before `TestMain` could configure the secret, killing all test suites; replaced with lazy `sync.Once` initialization

### Security
- 14 security findings addressed across all severity levels (Critical, High, Medium, Low)
- Generic rate limiting with in-memory fallback protects against brute-force even when Redis is unavailable
- Security headers protect against clickjacking, MIME sniffing, and XSS

---

## [1.0.0-alpha.3] - 2026-01-19

### Pre-Release Breaking Changes — Multi-Tenancy Support

This milestone introduced **multi-tenancy** architecture, enabling the API to serve multiple tenants and applications. These changes affect API clients upgrading from earlier pre-release builds.

#### Required API Changes

**All API requests now require the `X-App-ID` header:**

```bash
# Before (alpha.1 / alpha.2)
curl -X POST /auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"user@example.com","password":"secret"}'

# After (alpha.3+)
curl -X POST /auth/register \
  -H "Content-Type: application/json" \
  -H "X-App-ID: 00000000-0000-0000-0000-000000000001" \
  -d '{"email":"user@example.com","password":"secret"}'
```

**Exceptions (no `X-App-ID` required):**
- Swagger documentation endpoints (`/swagger/*`)
- Admin endpoints (`/admin/*`)
- OAuth callback endpoints (app_id in state parameter)

#### Migration Impact

**Database Schema:**
- New tables: `tenants`, `applications`, `oauth_provider_configs`
- Modified tables: `users`, `social_accounts`, `activity_logs` now include `app_id` foreign key
- Existing data automatically migrated to default tenant/application (`00000000-0000-0000-0000-000000000001`)
- Email uniqueness now scoped per application (was globally unique)

**JWT Tokens:**
- JWT tokens now include `app_id` claim
- Existing tokens issued before upgrade will be invalid
- Users must re-authenticate after migration

**OAuth Configuration:**
- OAuth provider credentials (Google, Facebook, GitHub) now configured per-application
- Use migration tool `cmd/migrate_oauth/main.go` to migrate existing credentials from environment variables to database
- Legacy environment-based OAuth config still supported for default app

### Added

#### Multi-Tenancy Architecture
- **Tenant Management**: Create and manage multiple tenant organizations via admin API
- **Application Management**: Each tenant can have multiple applications with isolated user bases
- **Per-Application OAuth**: OAuth providers (Google, Facebook, GitHub) configured per-application in database
- **Admin API Endpoints**:
  - `POST /admin/tenants` - Create tenant
  - `GET /admin/tenants` - List tenants (paginated)
  - `POST /admin/apps` - Create application
  - `GET /admin/apps` - List applications (paginated)
  - `POST /admin/oauth-providers` - Configure OAuth provider for app
  - `GET /admin/oauth-providers/:app_id` - List OAuth providers for app
  - `PUT /admin/oauth-providers/:id` - Update OAuth provider config
  - `DELETE /admin/oauth-providers/:id` - Delete OAuth provider config

#### New Middleware
- **AppID Middleware**: Validates and injects `X-App-ID` header into request context
- Query parameter fallback: `?app_id=<uuid>` supported when header not available

#### New Models
- **Tenant Model** (`pkg/models/tenant.go`): Organization-level entity
- **Application Model** (`pkg/models/application.go`): Tenant's application entity
- **OAuthProviderConfig Model** (`pkg/models/oauth_provider_config.go`): Per-app OAuth credentials

#### Migration Tools
- **Migration Script**: `migrations/20260105_add_multi_tenancy.sql` with automatic data migration
- **Rollback Script**: `migrations/20260105_add_multi_tenancy_rollback.sql` for safe rollback
- **OAuth Migration Tool**: `cmd/migrate_oauth/main.go` to migrate OAuth credentials from env to database
- **Enhanced Backup Scripts**: 
  - `scripts/backup_db.sh` (Unix/Mac)
  - `scripts/backup_db.bat` (Windows)
- **Migration Helper Scripts**:
  - `scripts/apply_pending_migrations.sh` - Apply pending migrations
  - `scripts/rollback_last_migration.sh` - Rollback last migration

#### Documentation
- **Copilot Instructions**: `.github/copilot-instructions.md` for AI-assisted development
- **Migration Documentation**: `migrations/20260105_add_multi_tenancy.md`
- **Admin API DTOs**: `pkg/dto/admin.go` with request/response structures
- **Updated Swagger**: Complete API documentation with new admin endpoints

### Changed

#### Authentication Flow
- JWT generation now includes `app_id` claim (see `pkg/jwt/jwt.go`)
- Auth middleware validates `app_id` from token against request header
- User lookup queries now scoped by `app_id`

#### Social Login
- OAuth state now includes `app_id` for callback routing
- Social account linkage scoped per application
- OAuth credentials loaded from database per-application (fallback to env vars for default app)

#### Two-Factor Authentication
- TOTP secrets and recovery codes now scoped per application
- 2FA state isolated between applications

#### User Management
- User registration scoped by `app_id`
- Email uniqueness constraint now `(email, app_id)` instead of global
- Profile endpoints return data scoped to request's `app_id`

#### Activity Logging
- All activity logs include `app_id` for audit trail segmentation
- Log queries filtered by application context

#### Testing
- All API tests updated to include `X-App-ID` header
- Test scripts (`test_api.sh`, `test_logout.sh`) updated with multi-tenancy support

#### Configuration
- Redis key prefixes now include `app_id` for session isolation
- CORS middleware allows `X-App-ID` header in requests

### Fixed
- Docker network creation documentation (`README.md`) - Added step to create shared network before starting containers

### Security
- **Data Isolation**: Complete tenant/application data isolation at database level
- **OAuth Security**: OAuth credentials stored per-application with encrypted secrets
- **JWT Claims**: App ID validation prevents cross-application token reuse
- **Index Updates**: Optimized indexes for multi-tenant queries

### Migration Guide

#### For Existing Installations (Upgrading from alpha.1/alpha.2)

**⚠️ CRITICAL: Backup your database before proceeding!**

1. **Backup Database**:
   ```bash
   make migrate-backup
   # or manually:
   pg_dump -U postgres -d auth_db > backup_$(date +%Y%m%d_%H%M%S).sql
   ```

2. **Apply Migration**:
   ```bash
   make migrate-up
   # This automatically:
   # - Creates tenants, applications, oauth_provider_configs tables
   # - Adds app_id columns to users, social_accounts, activity_logs
   # - Migrates all existing data to default tenant/app
   # - Updates indexes (email uniqueness now per-app)
   # - Records migration in schema_migrations table
   ```

3. **Migrate OAuth Credentials** (if using social login):
   ```bash
   go run cmd/migrate_oauth/main.go
   # Reads from .env and creates oauth_provider_configs entries
   # For providers: Google, Facebook, GitHub
   ```

4. **Update API Clients**:
   - Add `X-App-ID: 00000000-0000-0000-0000-000000000001` header to all requests
   - Default app ID is created automatically during migration
   - Update documentation/SDKs with new header requirement
   - Notify users to re-authenticate (existing JWTs are invalid)

5. **Test Endpoints**:
   ```bash
   # Test registration
   curl -X POST http://localhost:8080/auth/register \
     -H "X-App-ID: 00000000-0000-0000-0000-000000000001" \
     -H "Content-Type: application/json" \
     -d '{"email":"test@example.com","password":"Test123!@#"}'
   
   # Test login
   curl -X POST http://localhost:8080/auth/login \
     -H "X-App-ID: 00000000-0000-0000-0000-000000000001" \
     -H "Content-Type: application/json" \
     -d '{"email":"test@example.com","password":"Test123!@#"}'
   ```

6. **Rollback (if needed)**:
   ```bash
   # If migration fails or issues arise:
   make migrate-down
   # or manually:
   psql -U postgres -d auth_db -f migrations/20260105_add_multi_tenancy_rollback.sql
   
   # Restore from backup if necessary:
   psql -U postgres -d auth_db < backup_20260119_143022.sql
   ```

**Estimated Migration Time:**
- Small databases (<10k users): 1-2 minutes
- Medium databases (10k-100k users): 3-5 minutes
- Large databases (>100k users): 5-15 minutes

**Expected Downtime:** 5-15 minutes (application must be stopped during migration)

#### For New Installations

- Multi-tenancy enabled by default
- Default tenant and application created automatically
- Use admin API to create additional tenants/applications:
  ```bash
  # Create tenant
  POST /admin/tenants
  {"name": "Acme Corp"}
  
  # Create application
  POST /admin/apps
  {"tenant_id": "<tenant_id>", "name": "Mobile App", "description": "iOS/Android app"}
  
  # Configure OAuth for app
  POST /admin/oauth-providers
  {
    "app_id": "<app_id>",
    "provider": "google",
    "client_id": "xxx",
    "client_secret": "yyy",
    "redirect_url": "https://app.example.com/auth/google/callback",
    "is_enabled": true
  }
  ```

#### Troubleshooting

**Issue: "X-App-ID header is required" error**
- Solution: Add header to all API requests (except /swagger/* and /admin/*)
- Default app ID: `00000000-0000-0000-0000-000000000001`

**Issue: JWT tokens not working after migration**
- Solution: This is expected. Users must re-login to get new JWTs with `app_id` claim

**Issue: Social login not working**
- Solution: Run OAuth migration tool: `go run cmd/migrate_oauth/main.go`
- Or configure via Admin API: `POST /admin/oauth-providers`

**Issue: Email already exists error for different apps**
- Solution: This is correct behavior - email uniqueness is now per-app, not global

**Issue: Migration fails with foreign key error**
- Solution: Check database constraints. Rollback and restore from backup.

**Need Help?**
- See: [Pre-Release Migration Reference](docs/BREAKING_CHANGES.md) for detailed migration guide
- See: `migrations/20260105_add_multi_tenancy.md` for technical details
- Open GitHub issue with "migration-help" label

---

## [1.0.0-alpha.2] - 2024-12-04

### Added

#### CI/CD Improvements
- **GitHub Actions Workflow**: Complete CI/CD pipeline with test, build, and security-scan jobs
- **Local Testing Support**: Full compatibility with `act` for running GitHub Actions locally
- **Improved Port Configuration**: Services use non-conflicting ports (PostgreSQL: 5435, Redis: 6381) for CI
- **Smart Artifact Handling**: Conditional artifact upload/download based on environment (skips for local act runs)

#### Security Enhancements
- **Gosec Security Scanner**: Automated security scanning integrated into CI/CD
- **Nancy Vulnerability Scanner**: Optional dependency vulnerability scanning (requires authentication)
- **Security Exception Documentation**: Proper `#nosec` comments with justification for legitimate cases

#### Test Infrastructure
- **Environment Variable Support**: Tests now properly read from CI environment variables via `viper.AutomaticEnv()`
- **Redis Connection Handling**: Improved test reliability with proper Redis configuration
- **Test Coverage**: Maintained high test coverage across all components

#### Documentation
- **CI/CD Commands Section**: Added commands for running CI locally with act
- **Updated README**: Added CI/CD features to Developer Experience section
- **Installation Guide**: Added note about installing act for local CI testing

### Fixed
- Port conflicts in CI/CD workflow when running with `act` (changed from default 5432/6379 to 5435/6381)
- Test configuration to respect environment variables via `viper.AutomaticEnv()` and `viper.SetDefault()`
- Artifact operations now skip when running locally with `act` using `if: ${{ !env.ACT }}` condition
- Security scanner false positive for non-cryptographic random number usage in log sampling
- Nancy vulnerability scanner now uses `continue-on-error` to prevent CI failures when OSS Index authentication is not configured

### Changed
- CI workflow now uses different ports to avoid conflicts with local development environments
- Test setup uses `SetDefault` instead of `Set` to allow environment variable override

### Added - Professional Activity Logging System

#### Migration System & Documentation

#### Smart Logging with 80-95% Database Reduction
- **Event Severity Classification**: Events categorized as Critical, Important, or Informational
- **Intelligent Logging**: High-frequency events (TOKEN_REFRESH, PROFILE_ACCESS) disabled by default
- **Anomaly Detection**: Automatically logs unusual patterns (new IP, new device) even for disabled events
- **Automatic Cleanup**: Background service removes expired logs based on retention policies
- **Configurable Retention**: 
  - Critical events: 365 days (LOGIN, PASSWORD_CHANGE, 2FA changes)
  - Important events: 180 days (EMAIL_VERIFY, SOCIAL_LOGIN, PROFILE_UPDATE)
  - Informational events: 90 days (TOKEN_REFRESH, PROFILE_ACCESS when enabled)

#### New Configuration System
- Created `internal/config/logging.go` for centralized logging configuration
- All settings configurable via environment variables
- Event enable/disable controls per event type
- Sampling rates for high-frequency events
- Anomaly detection configuration
- Retention policies per severity level

#### Anomaly Detection Engine
- Created `internal/log/anomaly.go` for behavioral analysis
- Detects new IP addresses from user's historical patterns
- Detects new devices/browsers (user agent changes)
- Configurable pattern analysis window (default: 30 days)
- Optional unusual time access detection
- Privacy-preserving pattern storage (hashed IPs/user agents)

#### Enhanced Data Model
- Added `severity` field to activity_logs (CRITICAL, IMPORTANT, INFORMATIONAL)
- Added `expires_at` field for automatic expiration timestamps
- Added `is_anomaly` flag to identify anomaly-triggered logs
- Created composite indexes for efficient queries and cleanup
- Migration script with rollback capability

#### Comprehensive Migration System
- **[MIGRATIONS.md](docs/migrations/MIGRATIONS.md)**: User-friendly migration guide with step-by-step instructions
- **[BREAKING_CHANGES.md](docs/BREAKING_CHANGES.md)**: Breaking changes tracker with version history
- **[UPGRADE_GUIDE.md](docs/migrations/UPGRADE_GUIDE.md)**: Detailed version upgrade instructions with rollback procedures
- **[migrations/README.md](migrations/README.md)**: Developer-focused migration guide with best practices
- **[migrations/TEMPLATE.md](migrations/TEMPLATE.md)**: Standardized template for creating new migrations
- **[migrations/MIGRATIONS_LOG.md](migrations/MIGRATIONS_LOG.md)**: Historical log of all applied migrations

#### Migration Tools
- **scripts/migrate.sh**: Interactive Unix/Mac migration tool with:
  - Migration status checking
  - Automatic backups before migrations
  - Apply/rollback functionality
  - Database connection testing
- **scripts/migrate.bat**: Windows-compatible migration tool
- **Makefile targets**:
  - `make migrate` - Interactive migration tool
  - `make migrate-up` - Apply migrations
  - `make migrate-down` - Rollback migrations
  - `make migrate-status` - Check migration status
  - `make migrate-backup` - Create database backup

#### Contributor-Friendly Process
- Updated [CONTRIBUTING.md](CONTRIBUTING.md) with detailed migration guidelines
- Clear process for creating migrations with checklists
- Breaking change documentation requirements
- Testing and verification procedures
- Semver guidelines for version bumping

#### Automatic Cleanup Service
- Created `internal/log/cleanup.go` for background log deletion
- Runs on configurable schedule (default: daily)
- Batch processing to avoid database locks (default: 1000 per batch)
- Graceful shutdown handling
- Statistics tracking and manual trigger capability
- GDPR compliance support (delete user logs on request)

#### Comprehensive Documentation
- Created `docs/ACTIVITY_LOGGING_GUIDE.md` - Complete configuration guide
- Created `docs/ENV_VARIABLES.md` - All environment variables reference
- Created `docs/SMART_LOGGING_IMPLEMENTATION.md` - Implementation summary
- Created `migrations/README_SMART_LOGGING.md` - Migration instructions
- Updated `docs/API.md` with new event categorization
- Updated `README.md` with smart logging features

#### Configuration Examples
```bash
# Default behavior (zero configuration needed)
# - Critical/Important events: Always logged
# - TOKEN_REFRESH/PROFILE_ACCESS: Disabled (logged only on anomaly)
# - Automatic cleanup: Enabled (runs daily)
# - Anomaly detection: Enabled

# Optional customization via environment variables:
LOG_DISABLED_EVENTS=TOKEN_REFRESH,PROFILE_ACCESS
LOG_ANOMALY_DETECTION_ENABLED=true
LOG_RETENTION_CRITICAL=365
LOG_RETENTION_IMPORTANT=180
LOG_RETENTION_INFORMATIONAL=90
LOG_CLEANUP_ENABLED=true
LOG_CLEANUP_INTERVAL=24h
```

#### Expected Impact
- **Database Size**: 80-95% reduction in log volume
- **Performance**: Maintained (async logging) with enhanced indexes
- **Security**: Improved focus on actionable events with anomaly detection
- **Compliance**: Maintained for all critical audit requirements
- **Flexibility**: Fully configurable per deployment environment

#### Breaking Changes
- None - backward compatible with existing activity logs
- Existing logs automatically assigned severity and expiration on migration
- All existing API endpoints continue to work unchanged

#### Migration Required
- Run migration: `migrations/20240103_add_activity_log_smart_fields.sql`
- Rollback available: `migrations/20240103_add_activity_log_smart_fields_rollback.sql`
- See `migrations/README_SMART_LOGGING.md` for detailed instructions

### Added - Profile Sync on Social Login (2025-11-08)

#### Automatic Profile Synchronization
- **Profile data now automatically syncs** from social providers on every login
- System updates both `social_accounts` and `users` tables with latest provider data
- **Smart update strategy**: Only updates fields that have changed
- **Non-blocking**: Authentication succeeds even if profile update fails
- Supports all providers: Google, Facebook, GitHub

#### What Gets Synced
- Profile picture (avatar/photo URL)
- Full name, first name, last name
- Email from provider
- Locale/language preference
- Username (GitHub login, etc.)
- Complete raw provider response (JSONB)
- OAuth access token

#### Benefits
- Users see updated profile pictures immediately after changing them on social platforms
- Name changes on social accounts automatically reflected in app
- No manual sync or refresh needed
- Data stays current with social provider

#### Repository Enhancement
- Added `UpdateSocialAccount()` method to social repository
- Enables full social account record updates via GORM

#### Profile Endpoint Enhancement
- Updated `UserResponse` DTO to include all new profile fields
- Added `SocialAccountResponse` DTO for social account data
- Modified `GetUserByID` repository to preload social accounts
- Enhanced `GetProfile` handler to return complete user profile with social accounts
- Profile endpoint now returns: name, first_name, last_name, profile_picture, locale, social_accounts
- Regenerated Swagger documentation to reflect new profile structure

### Added - Social Login Data Enhancement (2025-11-08)

#### User Model Enhancements
- Added `Name` field to store full name from social login or user input
- Added `FirstName` field for first name from social login
- Added `LastName` field for last name from social login
- Added `ProfilePicture` field to store profile picture URL from social providers
- Added `Locale` field for user's language/locale preference

#### Social Account Model Enhancements
- Added `Email` field to store email from social provider
- Added `Name` field to store name from social provider
- Added `FirstName` field for first name from social provider
- Added `LastName` field for last name from social provider
- Added `ProfilePicture` field for profile picture URL from social provider
- Added `Username` field for username/login from providers (e.g., GitHub login)
- Added `Locale` field for locale from social provider
- Added `RawData` JSONB field to store complete raw JSON response from provider

#### Service Layer Enhancements
- Added `UpdateUser()` method to user repository for updating user profile data
- Enhanced Google login handler to capture: email, verified_email, name, given_name, family_name, picture, locale
- Enhanced Facebook login handler to capture: email, name, first_name, last_name, picture (large), locale
- Enhanced GitHub login handler to capture: email, name, login, avatar_url, bio, location, company
- Implemented smart profile update logic: only update user fields if currently empty when linking social accounts
- Store complete provider response in `RawData` field for all providers

#### API Changes
- Profile endpoint (`GET /profile`) now returns additional fields: name, first_name, last_name, profile_picture, locale
- Social account objects now include all new fields in responses
- No breaking changes - all new fields are optional and nullable

### Changed
- Modified social login data extraction to request extended fields from providers
- Updated Facebook Graph API call to request: `id,name,email,first_name,last_name,picture.type(large),locale`
- Enhanced social account linking to preserve and enrich existing user profile data

### Technical Details
- **Migration Method:** GORM AutoMigrate (automatic on application startup)
- **Database Impact:** Adds 5 columns to `users` table, 8 columns to `social_accounts` table
- **Backward Compatibility:** Fully backward compatible - all new fields are nullable
- **Files Modified:**
  - `pkg/models/user.go` - User model with new profile fields
  - `pkg/models/social_account.go` - Social account model with extended data fields
  - `internal/social/service.go` - Enhanced provider handlers for Google, Facebook, GitHub
  - `internal/user/repository.go` - Added UpdateUser method
  - `docs/migrations/MIGRATION_SOCIAL_LOGIN_DATA.md` - Migration documentation

### Documentation
- Added comprehensive migration documentation in `docs/migrations/MIGRATION_SOCIAL_LOGIN_DATA.md`
- Documents data flow changes, database schema updates, and testing recommendations
- Includes security considerations and rollback plan

---

## [1.0.0-alpha.1] - 2024-01-03

### Features
- User registration and authentication
- Email verification
- Password reset functionality
- Two-factor authentication (TOTP)
- Social login integration (Google, Facebook, GitHub)
- JWT-based authentication (access & refresh tokens)
- Activity logging
- Redis-based session management
- Comprehensive API documentation with Swagger

