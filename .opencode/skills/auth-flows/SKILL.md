---
name: auth-flows
description: Detailed documentation of the 4 authentication systems, token lifecycle, middleware pipeline, session management, and RBAC authorization.
license: MIT
---

## Authentication Systems Overview

The project has 4 independent authentication systems:

| System | Mechanism | Middleware File | Context Keys Set |
|--------|-----------|-----------------|------------------|
| User JWT | Bearer token | `internal/middleware/auth.go` | `userID`, `appID`, `roles`, `sessionID` |
| Admin API Key | Static env + DB keys | `internal/middleware/admin_auth.go` | `auth_type = "admin"` |
| App API Key | DB-backed per-app keys | `internal/middleware/app_api_key.go` | `auth_type = "app"` |
| Admin GUI Session | HTTP-only cookie + Redis | `internal/middleware/gui_auth.go` | `admin_id`, `admin_username`, `admin_session_id` |

## 1. User JWT Authentication

### Token Lifecycle

**Access Token:**
- Generated at login/social-login/2FA-verify/passkey-login/magic-link-verify
- Contains: UserID, AppID, SessionID, TokenType="access", Roles[]
- Expiry: 15 min default (configurable via `ACCESS_TOKEN_EXPIRATION_MINUTES`)
- Validated by `AuthMiddleware()` on every protected request

**Refresh Token:**
- Generated alongside access token
- Contains: UserID, AppID, SessionID, TokenType="refresh"
- Expiry: 720 hours default (configurable via `REFRESH_TOKEN_EXPIRATION_HOURS`)
- Used via `POST /refresh-token` to get a new access+refresh pair
- Old refresh token is rotated (new one issued, session updated)

**Token Blacklisting (Redis):**
- Individual access token blacklisting on logout
- User-wide token blacklisting on password change
- Session existence check (if session revoked, token rejected)

### AuthMiddleware Flow (`internal/middleware/auth.go`)

```
1. Extract "Authorization" header, strip "Bearer " prefix
2. Parse JWT via jwt.ParseToken()
3. Reject if TokenType is "refresh" (prevents refresh token misuse)
4. Check Redis: is this specific access token blacklisted?
5. Check Redis: are ALL tokens for this user blacklisted?
6. If claims.SessionID exists: check session still exists in Redis
7. Set context: userID, appID, roles, sessionID
```

### Login Flow (standard password login)

```
1. POST /login with email + password
2. userHandler.Login -> userService.Login
3. Verify password with bcrypt
4. Check if 2FA is enabled for user
   - If YES: return temp 2FA token, require POST /2fa/login-verify
   - If NO: generate access+refresh tokens, create session, return tokens
5. On success: activity log "USER_LOGIN"
```

### 2FA Login Flow

```
1. User logs in, gets temp 2FA token
2. POST /2fa/login-verify with temp token + code (TOTP/email/recovery)
   OR POST /2fa/passkey/begin + /finish for passkey 2FA
3. Verify code against:
   - TOTP: pquerna/otp library validation
   - Email: 6-digit code stored in Redis (10 min TTL)
   - Recovery: bcrypt-hashed codes stored in user record
   - Passkey: WebAuthn assertion ceremony
4. On success: generate real access+refresh tokens, create session
```

### Social Login Flow

```
1. GET /auth/{provider}/login -> redirect to OAuth provider
2. Provider redirects to GET /auth/{provider}/callback
3. Exchange code for tokens, fetch user profile
4. Find or create user (match by provider+provider_user_id+app_id)
5. Generate access+refresh tokens, create session
6. If 2FA enabled: same 2FA flow as above
```

### Magic Link Flow

```
1. POST /magic-link/request with email
2. Generate single-use token, store in Redis (10 min TTL)
3. Send email with magic link URL
4. POST /magic-link/verify with token
5. Validate token from Redis, delete after use
6. Generate access+refresh tokens, create session
```

### Passkey Passwordless Flow

```
1. POST /passkey/login/begin -> return WebAuthn challenge
2. Client performs assertion with authenticator
3. POST /passkey/login/finish with assertion response
4. Validate credential, look up user
5. Generate access+refresh tokens, create session
```

## 2. Admin API Key Authentication

### AdminAuthMiddleware Flow (`internal/middleware/admin_auth.go`)

```
1. Read X-Admin-API-Key header
2. Fast path: compare against ADMIN_API_KEY env var (constant-time)
3. Fallback: SHA-256 hash the key, look up in api_keys table
   - Validate key_type == "admin"
   - Async update last_used_at
4. Set context: auth_type = "admin"
```

### Key Format
- Prefix: `ak_` + 24 random bytes base64-encoded
- Storage: SHA-256 hash in DB, prefix (8 chars) + suffix (4 chars) for display
- Generation: `internal/admin/apikey_util.go`

## 3. Per-Application API Key Authentication

### AppApiKeyMiddleware Flow (`internal/middleware/app_api_key.go`)

```
1. Read X-App-API-Key header
2. Read app_id from context (set by AppIDMiddleware)
3. SHA-256 hash the key, look up in api_keys table
4. Validate key_type == "app"
5. Validate key.AppID matches context app_id (cross-app prevention)
6. Async update last_used_at
7. Set context: auth_type = "app"
```

### AppRouteGuardMiddleware (`internal/middleware/app_route_guard.go`)

Additional guard that ensures the `:id` URL parameter matches the `X-App-ID` header, preventing URL manipulation attacks on `/app/:id/*` routes.

## 4. Admin GUI Session Authentication

### GUIAuthMiddleware Flow (`internal/middleware/gui_auth.go`)

```
1. Read "admin_session" cookie
2. Call sessionValidator.ValidateSession(sessionID) -> *AdminAccount
3. If invalid: clear cookie, redirect to /gui/login
4. Set context: admin_id, admin_username, admin_session_id
```

### Admin Login Flow

```
1. POST /gui/login with username + password
2. accountService.Authenticate(username, password) via bcrypt
3. If 2FA enabled:
   - Create pending 2FA session in Redis
   - Redirect to /gui/2fa-verify
   - POST /gui/2fa-verify with code
   - Promote pending session to full session
4. If no 2FA:
   - Create full session in Redis
   - Set admin_session cookie (HttpOnly, SameSite=Strict, Secure)
```

### Admin Sessions (Redis)

- Stored in Redis (not JWT-based)
- Session ID is a UUID stored in cookie
- Session data: admin account reference
- CSRF token generated per session
- Separate from user sessions

### CSRF Protection (`internal/middleware/csrf.go`)

```
Safe methods (GET/HEAD/OPTIONS):
  -> Generate CSRF token, store in context for template rendering

State-changing methods (POST/PUT/DELETE):
  -> Read token from X-CSRF-Token header (HTMX) OR _csrf form field
  -> Validate against session's stored token
  -> 403 on mismatch
```

## RBAC Authorization

### Permission Model

Permissions are `resource:action` pairs (e.g., `user:read`, `settings:write`).

### AuthorizePermission Middleware (`internal/middleware/auth.go`)

```
1. Read userID and appID from context
2. Call rbacService.HasPermission(appID, userID, resource, action)
3. RBAC service checks Redis cache first, then DB
4. Cache key: rbac:{appID}:{userID}
5. Cache TTL: matches access token TTL
6. Returns 403 on failure
```

### Default Roles (seeded per new app)

| Role | Permissions |
|------|-------------|
| admin | All permissions |
| member | Read + limited write |
| viewer | Read-only |

### Permission Resources Used

`user`, `settings`, `log` -- used with actions `read`, `write`, `delete`.

## Session Management

### User Sessions (`internal/session/service.go`)

**Redis storage:**
- `session:{appID}:{sessionID}` -- hash with user_id, refresh_token, ip, user_agent, created_at, last_active
- `user_sessions:{appID}:{userID}` -- set of session IDs

**Lifecycle:**
- Created at login (with refresh token)
- Refreshed on token rotation (new refresh token stored)
- Revoked individually or all-at-once
- Max sessions enforced (oldest evicted)

### Token Blacklisting (`internal/redis/redis.go`)

- `blacklist:access:{tokenHash}` -- individual token blacklist (TTL = remaining token life)
- `blacklist:user:{userID}` -- user-wide blacklist (TTL = access token TTL)

## AppID Extraction (`internal/middleware/app_id.go`)

```
1. Skip for /swagger, /admin, /gui paths
2. Read X-App-ID header
3. Fallback: app_id query parameter
4. Skip for /auth/*/callback (OAuth state carries app_id)
5. Parse as UUID, set in context
Default: 00000000-0000-0000-0000-000000000001
```

## Shared Interfaces (`web/context_keys.go`)

```go
type SessionValidator interface {
    ValidateSession(sessionID string) (*models.AdminAccount, error)
    GenerateCSRFToken(sessionID string) (string, error)
    ValidateCSRFToken(sessionID, token string) bool
}

type ApiKeyValidator interface {
    FindActiveKeyByHash(keyHash string) (*models.ApiKey, error)
    UpdateApiKeyLastUsed(id uuid.UUID)
}
```

## When To Use This Skill

Load this skill when working on authentication, authorization, middleware, token handling, session management, or any security-related feature.
