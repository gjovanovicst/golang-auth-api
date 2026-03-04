---
name: route-map
description: Complete HTTP route structure with auth layers, middleware chains, rate limiting, and handler mappings for all API and GUI endpoints.
license: MIT
---

## Route Architecture

All routes are defined in `cmd/api/main.go`. The server uses Gin with these global middleware applied to all routes:
1. `middleware.SecurityHeadersMiddleware()` -- CSP, HSTS, X-Frame-Options
2. `middleware.CORSMiddleware()` -- CORS headers
3. `middleware.AppIDMiddleware()` -- Extracts `X-App-ID` header (skips `/swagger`, `/admin`, `/gui`)

## Authentication Layers (4 systems)

| Layer | Mechanism | Header/Cookie | Middleware | Scope |
|-------|-----------|---------------|------------|-------|
| User Auth | JWT Bearer | `Authorization: Bearer <token>` | `AuthMiddleware()` | End-user API routes |
| Admin API Key | API key | `X-Admin-API-Key` | `AdminAuthMiddleware(adminRepo)` | `/admin/*` JSON API |
| App API Key | API key | `X-App-API-Key` + `X-App-ID` | `AppApiKeyMiddleware(adminRepo)` | `/app/:id/*` routes |
| Admin GUI Session | HTTP-only cookie | `admin_session` cookie | `GUIAuthMiddleware(accountService)` | `/gui/*` HTML interface |

## Public Routes (no auth, rate limited)

```
POST /register                    -> userHandler.Register           [APIRegisterRateLimit: 3/min]
POST /login                       -> userHandler.Login              [APILoginRateLimit: 5/min, lockout 10->15min]
POST /refresh-token               -> userHandler.RefreshToken       [APIRefreshTokenRateLimit: 10/min]
POST /forgot-password             -> userHandler.ForgotPassword     [APIForgotPasswordRateLimit: 3/min]
POST /reset-password              -> userHandler.ResetPassword      [APIResetPasswordRateLimit: 5/min]
GET  /verify-email                -> userHandler.VerifyEmail
POST /resend-verification         -> userHandler.ResendVerification [APIResendVerificationRateLimit: 3/min]

POST /2fa/login-verify            -> twofaHandler.VerifyLogin       [API2FAVerifyRateLimit: 5/min, lockout 10->15min]
POST /2fa/email/resend            -> twofaHandler.ResendEmail2FACode [API2FAVerifyRateLimit]
GET  /2fa/methods                 -> twofaHandler.GetAvailableMethods

POST /2fa/passkey/begin           -> webauthnHandler.BeginPasskey2FA    [APIPasskey2FARateLimit: 10/min]
POST /2fa/passkey/finish          -> webauthnHandler.FinishPasskey2FA   [APIPasskey2FARateLimit]

POST /passkey/login/begin         -> webauthnHandler.BeginPasswordlessLogin  [APIPasskeyLoginRateLimit: 10/min]
POST /passkey/login/finish        -> webauthnHandler.FinishPasswordlessLogin [APIPasskeyLoginRateLimit]

POST /magic-link/request          -> userHandler.RequestMagicLink    [APIMagicLinkRateLimit: 5/15min]
POST /magic-link/verify           -> userHandler.VerifyMagicLink     [APIMagicLinkRateLimit]
```

## Social OAuth2 Routes (public, no rate limit)

```
GET /auth/google/login            -> socialHandler.GoogleLogin
GET /auth/google/callback         -> socialHandler.GoogleCallback
GET /auth/facebook/login          -> socialHandler.FacebookLogin
GET /auth/facebook/callback       -> socialHandler.FacebookCallback
GET /auth/github/login            -> socialHandler.GithubLogin
GET /auth/github/callback         -> socialHandler.GithubCallback

# Account linking callbacks (public -- user ID in OAuth state param)
GET /auth/google/link/callback    -> socialHandler.GoogleLinkCallback
GET /auth/facebook/link/callback  -> socialHandler.FacebookLinkCallback
GET /auth/github/link/callback    -> socialHandler.GithubLinkCallback
```

## Account Linking Routes (JWT auth required)

```
GET /auth/google/link             -> socialHandler.GoogleLink       [AuthMiddleware]
GET /auth/facebook/link           -> socialHandler.FacebookLink     [AuthMiddleware]
GET /auth/github/link             -> socialHandler.GithubLink       [AuthMiddleware]
```

## Protected User Routes (JWT auth + RBAC permissions)

All routes use `AuthMiddleware()`. RBAC permissions shown as `resource:action`.

```
# Profile (user:read, user:write, user:delete)
GET    /profile                   -> userHandler.GetProfile         [user:read]
PUT    /profile                   -> userHandler.UpdateProfile      [user:write]
DELETE /profile                   -> userHandler.DeleteAccount      [user:delete]
PUT    /profile/email             -> userHandler.UpdateEmail        [user:write]
PUT    /profile/password          -> userHandler.UpdatePassword     [user:write]

# Social accounts (user:read, user:write)
GET    /profile/social-accounts   -> socialHandler.ListSocialAccounts   [user:read]
DELETE /profile/social-accounts/:id -> socialHandler.UnlinkSocialAccount [user:write]

# Auth (no extra permission)
GET  /auth/validate               -> userHandler.ValidateToken
POST /logout                      -> userHandler.Logout

# 2FA management (settings:write, settings:read)
POST /2fa/generate                -> twofaHandler.Generate2FA       [settings:write]
POST /2fa/verify-setup            -> twofaHandler.VerifySetup       [settings:write]
POST /2fa/enable                  -> twofaHandler.Enable2FA         [settings:write]
POST /2fa/disable                 -> twofaHandler.Disable2FA        [settings:write]
POST /2fa/recovery-codes          -> twofaHandler.GenerateRecoveryCodes [settings:write]
POST /2fa/email/enable            -> twofaHandler.EnableEmail2FA    [settings:write]

# Passkey management (settings:write, settings:read)
POST   /passkey/register/begin    -> webauthnHandler.BeginRegistration   [settings:write]
POST   /passkey/register/finish   -> webauthnHandler.FinishRegistration  [settings:write]
GET    /passkeys                  -> webauthnHandler.ListCredentials     [settings:read]
PUT    /passkeys/:id              -> webauthnHandler.RenameCredential    [settings:write]
DELETE /passkeys/:id              -> webauthnHandler.DeleteCredential    [settings:write]

# Activity logs (log:read)
GET /activity-logs                -> logHandler.GetUserActivityLogs      [log:read]
GET /activity-logs/event-types    -> logHandler.GetEventTypes            [log:read]
GET /activity-logs/:id            -> logHandler.GetActivityLogByID       [log:read]

# Sessions (no extra permission)
GET    /sessions                  -> sessionHandler.ListSessions
DELETE /sessions/:id              -> sessionHandler.RevokeSession
DELETE /sessions                  -> sessionHandler.RevokeAllSessions
```

## Admin API Routes (Admin API Key auth)

All routes use `AdminAuthMiddleware(adminRepo)`. Header: `X-Admin-API-Key`.

```
# Activity logs
GET /admin/activity-logs          -> logHandler.GetAllActivityLogs

# Tenants
POST /admin/tenants               -> adminHandler.CreateTenant
GET  /admin/tenants               -> adminHandler.ListTenants

# Applications
POST /admin/apps                  -> adminHandler.CreateApp
GET  /admin/apps/:id              -> adminHandler.GetAppDetails
POST /admin/apps/:id/oauth-config -> adminHandler.UpsertOAuthConfig

# Email types
GET    /admin/email-types         -> adminHandler.ListEmailTypes
GET    /admin/email-types/:code   -> adminHandler.GetEmailType
POST   /admin/email-types         -> adminHandler.CreateEmailType
PUT    /admin/email-types/:id     -> adminHandler.UpdateEmailType
DELETE /admin/email-types/:id     -> adminHandler.DeleteEmailType

# Email templates
GET    /admin/email-templates     -> adminHandler.ListEmailTemplates
GET    /admin/email-templates/:id -> adminHandler.GetEmailTemplate
POST   /admin/email-templates     -> adminHandler.SaveEmailTemplate
DELETE /admin/email-templates/:id -> adminHandler.DeleteEmailTemplate
POST   /admin/email-templates/preview -> adminHandler.PreviewEmailTemplate

# Email variables
GET /admin/email-variables        -> adminHandler.ListWellKnownVariables

# Email server configs (per-app)
GET    /admin/apps/:id/email-config    -> adminHandler.GetEmailServerConfig
PUT    /admin/apps/:id/email-config    -> adminHandler.SaveEmailServerConfig
DELETE /admin/apps/:id/email-config    -> adminHandler.DeleteEmailServerConfig
POST   /admin/apps/:id/email-test      -> adminHandler.SendTestEmail
GET    /admin/apps/:id/email-servers   -> adminHandler.ListEmailServerConfigsByApp

# Email server configs (global CRUD)
GET    /admin/email-servers            -> adminHandler.ListAllEmailServerConfigs
GET    /admin/email-servers/:id        -> adminHandler.GetEmailServerConfigByID
POST   /admin/email-servers            -> adminHandler.CreateEmailServerConfig
PUT    /admin/email-servers/:id        -> adminHandler.UpdateEmailServerConfigByID
DELETE /admin/email-servers/:id        -> adminHandler.DeleteEmailServerConfigByID
POST   /admin/email-servers/:id/test   -> adminHandler.SendTestEmailByConfigID

# Send email
POST /admin/apps/:id/send-email  -> adminHandler.SendCustomEmail

# RBAC
GET    /admin/rbac/roles          -> rbacHandler.ListRoles
GET    /admin/rbac/roles/:id      -> rbacHandler.GetRole
POST   /admin/rbac/roles          -> rbacHandler.CreateRole
PUT    /admin/rbac/roles/:id      -> rbacHandler.UpdateRole
DELETE /admin/rbac/roles/:id      -> rbacHandler.DeleteRole
PUT    /admin/rbac/roles/:id/permissions -> rbacHandler.SetRolePermissions
GET    /admin/rbac/permissions    -> rbacHandler.ListPermissions
POST   /admin/rbac/permissions    -> rbacHandler.CreatePermission
GET    /admin/rbac/user-roles     -> rbacHandler.ListUserRoles
POST   /admin/rbac/user-roles     -> rbacHandler.AssignRole
DELETE /admin/rbac/user-roles     -> rbacHandler.RevokeRole
GET    /admin/rbac/user-roles/user -> rbacHandler.GetUserRoles
```

## App API Routes (Per-App API Key auth)

Middleware chain: `AppApiKeyMiddleware(adminRepo)` + `AppRouteGuardMiddleware()`.
Headers: `X-App-API-Key` + `X-App-ID` (must match `:id` in URL).

```
GET  /app/:id/email-config        -> adminHandler.GetEmailServerConfig
GET  /app/:id/email-servers       -> adminHandler.ListEmailServerConfigsByApp
POST /app/:id/email-test          -> adminHandler.SendTestEmail
POST /app/:id/send-email          -> adminHandler.SendCustomEmail
```

## GUI Routes (Admin Web Interface)

Static assets and login pages are public. Authenticated routes use `GUIAuthMiddleware` + `CSRFMiddleware`.

### Public GUI routes
```
GET  /gui/static/*                -> static.HTTPFileSystem()
GET  /gui/login                   -> guiHandler.LoginPage
POST /gui/login                   -> guiHandler.LoginSubmit        [LoginRateLimit: 5/min]
POST /gui/passkey-login/begin     -> guiHandler.PasskeyLoginBegin  [GUIPasskeyLoginRateLimit: 10/min]
POST /gui/passkey-login/finish    -> guiHandler.PasskeyLoginFinish [GUIPasskeyLoginRateLimit]
POST /gui/magic-link-login        -> guiHandler.MagicLinkLoginRequest [GUIMagicLinkRateLimit: 3/15min]
GET  /gui/magic-link-login/verify -> guiHandler.MagicLinkLoginVerify
GET  /gui/2fa-verify              -> guiHandler.TwoFAVerifyPage
POST /gui/2fa-verify              -> guiHandler.TwoFAVerifySubmit
POST /gui/2fa-resend-email        -> guiHandler.TwoFAResendEmail
```

### Authenticated GUI routes (cookie session + CSRF)

Covers: Dashboard, Tenants, Applications, OAuth, Users, Logs, API Keys, Settings, Email Servers, Email Templates, Email Types, Roles, Permissions, User Roles, Sessions, My Account (email, password, 2FA, passkeys, magic link), Social Account/Passkey management for users.

Each entity follows the HTMX CRUD pattern:
```
GET  /gui/<entity>                -> Page (full page)
GET  /gui/<entity>/list           -> List (HTMX partial, paginated)
GET  /gui/<entity>/new            -> CreateForm (HTMX partial)
POST /gui/<entity>                -> Create
GET  /gui/<entity>/form-cancel    -> FormCancel (HTMX partial)
GET  /gui/<entity>/:id/edit       -> EditForm (HTMX partial)
PUT  /gui/<entity>/:id            -> Update
GET  /gui/<entity>/:id/delete     -> DeleteConfirm (HTMX partial)
DELETE /gui/<entity>/:id          -> Delete
```

## Rate Limiting Summary

| Endpoint Group | Prefix | Limit | Window | Lockout |
|----------------|--------|-------|--------|---------|
| GUI Login | `gui:login` | 5/min | 60s | 10 attempts -> 15min lockout |
| API Login | `api:login` | 5/min | 60s | 10 -> 15min |
| API Register | `api:register` | 3/min | 60s | none |
| API Forgot Password | `api:forgot-password` | 3/min | 60s | none |
| API Resend Verification | `api:resend-verification` | 3/min | 60s | none |
| API Refresh Token | `api:refresh-token` | 10/min | 60s | none |
| API Reset Password | `api:reset-password` | 5/min | 60s | none |
| API 2FA Verify | `api:2fa-verify` | 5/min | 60s | 10 -> 15min |
| API Passkey Login | `api:passkey-login` | 10/min | 60s | 20 -> 15min |
| API Passkey 2FA | `api:passkey-2fa` | 10/min | 60s | 20 -> 15min |
| API Magic Link | `api:magic-link` | 5/15min | 15min | none |
| GUI Magic Link | `gui:magic-link` | 3/15min | 15min | none |
| GUI Passkey Login | `gui:passkey-login` | 10/min | 60s | 20 -> 15min |

## When To Use This Skill

Load this skill when working on API endpoints, adding new routes, modifying middleware chains, changing rate limits, or debugging request flows.
