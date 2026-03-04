---
name: project-map
description: Complete module inventory of the Auth API project with file paths, dependencies, and architecture overview. Load this first in any new session.
license: MIT
---

## What This Project Is

A **multi-tenant authentication and authorization REST API** in Go 1.23+, using Gin, PostgreSQL (GORM), and Redis. It provides user registration, login, OAuth2 social login, TOTP and email-based 2FA, WebAuthn/passkeys, magic links, RBAC, session management, and a full admin interface (JSON API + HTMX GUI).

## Architecture

**Pattern:** Repository -> Service -> Handler (Clean Architecture)

**Key design decisions:**
- Concrete struct dependency injection (no interface-based DI containers)
- Function-type callbacks between modules to avoid import cycles (e.g., `RoleLookupFunc`, `AssignDefaultRoleFunc`)
- Multi-tenancy: **Tenant -> Application -> User** hierarchy
- Every user is scoped to an `AppID` (UUID), with default app `00000000-0000-0000-0000-000000000001`

## Entry Point

- `cmd/api/main.go` -- Dependency injection, route setup, server startup

## Domain Modules (internal/)

### internal/admin/ (11 files)

Tenant/app CRUD, OAuth provider configuration, email management, API key management, admin account authentication, 2FA for admin accounts, system settings (env -> DB -> default resolution), dashboard stats, HTMX GUI handler.

| File | Purpose |
|------|---------|
| `gui_handler.go` | HTMX admin GUI (4976 lines, largest file) |
| `handler.go` | Admin JSON REST API (1146 lines) |
| `repository.go` | Data access for tenants, apps, OAuth, users, API keys, logs (952 lines) |
| `account_service.go` | Admin auth, sessions, 2FA, CSRF, password ops |
| `account_repository.go` | AdminAccount GORM queries |
| `dashboard_service.go` | Dashboard stats aggregation (PostgreSQL + Redis) |
| `settings_service.go` | System settings with 3-tier resolution (env > DB > default) |
| `settings_repository.go` | SystemSetting GORM queries with upsert |
| `apikey_util.go` | API key generation (SHA-256 hash, prefix/suffix) |
| `apikey_util_test.go` | Tests for API key utilities |
| `account_service_test.go` | Tests for admin account service |

### internal/user/ (5 files)

Registration, login (with self-healing role assignment), password reset, magic link auth, email change with verification, profile management.

| File | Purpose |
|------|---------|
| `handler.go` | User HTTP handlers |
| `service.go` | User business logic |
| `repository.go` | User GORM queries |
| `handler_test.go` | Handler tests |
| `service_test.go` | Service tests |

### internal/social/ (4 files)

Google, Facebook, GitHub OAuth2 flows. Supports account linking and direct social login/registration.

| File | Purpose |
|------|---------|
| `handler.go` | OAuth2 HTTP handlers (login, callback, link) |
| `service.go` | OAuth2 business logic |
| `repository.go` | SocialAccount GORM queries |
| `oauth_state.go` | OAuth state parameter encoding (appID, provider, action, HMAC) |

### internal/twofa/ (2 files)

TOTP (authenticator apps), email-based 2FA codes, recovery codes.

| File | Purpose |
|------|---------|
| `handler.go` | 2FA HTTP handlers |
| `service.go` | 2FA business logic (TOTP + email codes) |

### internal/webauthn/ (5 files)

Full WebAuthn support for users and admin accounts.

| File | Purpose |
|------|---------|
| `handler.go` | Passkey HTTP handlers |
| `service.go` | WebAuthn ceremony logic |
| `repository.go` | WebAuthnCredential GORM queries |
| `config.go` | Relying party configuration |
| `user_adapter.go` | Adapts User/AdminAccount to WebAuthn user interface |

### internal/session/ (2 files)

Redis-backed sessions with refresh token rotation, multi-device tracking.

| File | Purpose |
|------|---------|
| `service.go` | Session lifecycle (create, refresh, revoke, list) |
| `handler.go` | Session API endpoints |

### internal/rbac/ (3 files)

Roles per-application, permissions as `resource:action`, Redis-cached authorization.

| File | Purpose |
|------|---------|
| `service.go` | RBAC logic with Redis caching |
| `repository.go` | Role/Permission/UserRole GORM queries |
| `handler.go` | RBAC API endpoints |

### internal/email/ (8 files)

Multi-layered email system: Service -> VariableResolver + Renderer + Sender.

| File | Purpose |
|------|---------|
| `service.go` | Orchestrator (send pipeline, template/SMTP resolution) |
| `resolver.go` | Variable resolution pipeline (4 layers) |
| `renderer.go` | Three template engines (go_template, placeholder, raw_html) |
| `sender.go` | SMTP sending via gopkg.in/mail.v2 |
| `types.go` | Constants, structs, variable registry |
| `defaults.go` | 7 hardcoded default email templates |
| `repository.go` | Email types, templates, server configs GORM queries |
| `email_integration_test.go` | Integration tests |

### internal/log/ (6 files)

Async channel-based logging with anomaly detection.

| File | Purpose |
|------|---------|
| `service.go` | Async log service (buffered channel, background worker) |
| `anomaly.go` | Anomaly detection (new IP, new UA, unusual time) |
| `cleanup.go` | Scheduled log retention/cleanup |
| `query_service.go` | Filtered, paginated log querying |
| `handler.go` | Log API endpoints |
| `repository.go` | ActivityLog GORM queries |

### internal/middleware/ (13 files)

| File | Purpose |
|------|---------|
| `auth.go` | JWT auth + token blacklist checking |
| `admin_auth.go` | Admin API Key auth (static env + DB-backed) |
| `app_api_key.go` | Per-app API Key auth |
| `gui_auth.go` | Admin GUI cookie session auth |
| `csrf.go` | CSRF protection for GUI |
| `app_id.go` | X-App-ID header extraction |
| `app_route_guard.go` | Cross-app URL parameter validation |
| `rate_limit.go` | Redis + in-memory fallback rate limiting |
| `cors.go` | CORS configuration |
| `security_headers.go` | CSP, HSTS, X-Frame-Options |
| `auth_test.go` | Auth middleware tests |
| `rate_limit_test.go` | Rate limit tests |
| `security_headers_test.go` | Security header tests |

### Other internal packages

| Package | File | Purpose |
|---------|------|---------|
| `internal/database/` | `db.go` | PostgreSQL connection + GORM auto-migration |
| `internal/redis/` | `redis.go` | Redis connection + token blacklisting + session helpers |
| `internal/config/` | `logging.go` | Logging configuration |
| `internal/util/` | `client_info.go` | Client info extraction from requests |

## Shared Packages (pkg/)

| Package | Files | Purpose |
|---------|-------|---------|
| `pkg/models/` | 14 model files | 17 GORM models (User, Tenant, Application, Role, Permission, UserRole, AdminAccount, SocialAccount, WebAuthnCredential, ActivityLog, ApiKey, EmailType, EmailTemplate, EmailServerConfig, OAuthProviderConfig, SystemSetting, SchemaMigration) |
| `pkg/dto/` | 7 files | Request/response DTOs: auth, admin, session, RBAC, WebAuthn, email, activity_log |
| `pkg/errors/` | `errors.go`, `errors_test.go` | AppError type with 6 HTTP status code mappings |
| `pkg/jwt/` | `jwt.go`, `jwt_test.go` | JWT Claims (UserID, AppID, SessionID, TokenType, Roles), generate/parse |

## Web Package (web/)

| File | Purpose |
|------|---------|
| `renderer.go` | HTML template renderer (embedded templates, funcMap) |
| `context_keys.go` | Shared context keys, SessionValidator/ApiKeyValidator interfaces, cookie helpers |
| `static/embed.go` | Embedded static files (CSS/JS) |

## Dependencies Between Modules

```
main.go wires everything:
  user.Service depends on: user.Repository, email.Service, rbac.Service (via callbacks), session.Service
  social.Service depends on: user.Repository, social.Repository, rbac.Service (via callbacks), session.Service
  twofa.Service depends on: user.Repository, email.Service
  twofa.Handler depends on: rbac.Service (via callbacks), session.Service
  webauthn.Service depends on: webauthn.Repository, user.Repository
  webauthn.Handler depends on: rbac.Service (via callbacks), session.Service
  rbac.Service depends on: rbac.Repository
  session.Service depends on: Redis
  email.Service depends on: email.Repository, VariableResolver, Renderer, Sender
  log.Service depends on: log.Repository, AnomalyDetector
  admin.Handler depends on: admin.Repository, email.Service
  admin.GUIHandler depends on: AccountService, DashboardService, admin.Repository, SettingsService, email.Service, rbac.Service, webauthn.Service
```

## When To Use This Skill

Load this skill at the start of any session to understand the project structure. For domain-specific deep dives, also load the relevant skill: `route-map`, `data-model`, `auth-flows`, `email-system`, or `admin-gui`.
