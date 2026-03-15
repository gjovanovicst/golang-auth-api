# API Endpoints

All endpoints (except `/swagger/*`, `/admin/*`, and OAuth callbacks) require the `X-App-ID` header.

Interactive documentation is available at [http://localhost:8080/swagger/index.html](http://localhost:8080/swagger/index.html) when the server is running.

For detailed request/response schemas, see [API.md](API.md).

---

## Health & Metrics

| Endpoint | Method | Description | Auth |
|----------|--------|-------------|------|
| `/health` | GET | Liveness check — database, Redis, and SMTP reachability | No |
| `/metrics` | GET | Prometheus metrics (request counters, system info) | Admin API Key |

---

## Admin API

| Endpoint | Method | Description | Auth |
|----------|--------|-------------|------|
| `/admin/tenants` | POST | Create new tenant | Admin |
| `/admin/tenants` | GET | List all tenants (paginated) | Admin |
| `/admin/apps` | POST | Create application for tenant | Admin |
| `/admin/apps` | GET | List applications (paginated) | Admin |
| `/admin/oauth-providers` | POST | Configure OAuth provider for app | Admin |
| `/admin/oauth-providers/:app_id` | GET | List OAuth providers for app | Admin |
| `/admin/oauth-providers/:id` | PUT | Update OAuth provider config | Admin |
| `/admin/oauth-providers/:id` | DELETE | Delete OAuth provider config | Admin |
| `/admin/users/export` | GET | Export all users as CSV | Admin |
| `/admin/users/import` | POST | Bulk-import users from CSV | Admin |
| `/admin/users/:id/trusted-devices` | GET | List trusted devices for a user | Admin |
| `/admin/users/:id/trusted-devices` | DELETE | Revoke all trusted devices for a user | Admin |
| `/admin/activity-logs/export` | GET | Export activity logs as CSV | Admin |

### IP Rules (per application)

| Endpoint | Method | Description | Auth |
|----------|--------|-------------|------|
| `/admin/apps/:id/ip-rules` | GET | List IP rules for an application | Admin |
| `/admin/apps/:id/ip-rules` | POST | Create an IP rule (CIDR or country) | Admin |
| `/admin/apps/:id/ip-rules/:rule_id` | GET | Get a specific IP rule | Admin |
| `/admin/apps/:id/ip-rules/:rule_id` | PUT | Update an IP rule | Admin |
| `/admin/apps/:id/ip-rules/:rule_id` | DELETE | Delete an IP rule | Admin |
| `/admin/apps/:id/ip-rules/check` | POST | Test whether an IP is allowed | Admin |

### Webhooks

| Endpoint | Method | Description | Auth |
|----------|--------|-------------|------|
| `/admin/webhooks` | GET | List all webhook endpoints (all apps) | Admin |
| `/admin/webhooks/apps/:app_id` | GET | List webhook endpoints for an app | Admin |
| `/admin/webhooks/apps/:app_id` | POST | Create a webhook endpoint | Admin |
| `/admin/webhooks/:id/toggle` | PUT | Enable or disable a webhook endpoint | Admin |
| `/admin/webhooks/:id` | DELETE | Delete a webhook endpoint | Admin |
| `/admin/webhooks/:id/deliveries` | GET | List delivery history for a webhook | Admin |
| `/admin/webhooks/apps/:app_id/deliveries` | GET | List all deliveries for an app | Admin |
| `/app/:id/webhooks` | GET | List webhook endpoints (App API Key) | App API Key |
| `/app/:id/webhooks` | POST | Create a webhook endpoint (App API Key) | App API Key |
| `/app/:id/webhooks/:wid/toggle` | PUT | Toggle a webhook endpoint (App API Key) | App API Key |
| `/app/:id/webhooks/:wid` | DELETE | Delete a webhook endpoint (App API Key) | App API Key |
| `/app/:id/webhooks/:wid/deliveries` | GET | List delivery history (App API Key) | App API Key |

### OIDC Client Management

| Endpoint | Method | Description | Auth |
|----------|--------|-------------|------|
| `/admin/oidc/apps/:id/clients` | GET | List OIDC relying-party clients | Admin |
| `/admin/oidc/apps/:id/clients` | POST | Register a new OIDC client | Admin |
| `/admin/oidc/apps/:id/clients/:cid` | GET | Get OIDC client details | Admin |
| `/admin/oidc/apps/:id/clients/:cid` | PUT | Update OIDC client | Admin |
| `/admin/oidc/apps/:id/clients/:cid` | DELETE | Delete OIDC client | Admin |
| `/admin/oidc/apps/:id/clients/:cid/rotate-secret` | POST | Rotate client secret | Admin |

---

## Authentication

| Endpoint | Method | Description | Auth |
|----------|--------|-------------|------|
| `/register` | POST | User registration | No |
| `/login` | POST | User login (with 2FA support) | No |
| `/logout` | POST | Logout and token revocation | Yes |
| `/refresh-token` | POST | Refresh JWT tokens | No |
| `/verify-email` | GET | Email verification | No |
| `/resend-verification` | POST | Resend email verification | No |
| `/forgot-password` | POST | Request password reset | No |
| `/reset-password` | POST | Reset password with token | No |

---

## Two-Factor Authentication (2FA)

| Endpoint | Method | Description | Auth |
|----------|--------|-------------|------|
| `/2fa/generate` | POST | Generate TOTP secret and QR code | Yes |
| `/2fa/verify-setup` | POST | Verify initial TOTP setup | Yes |
| `/2fa/enable` | POST | Enable TOTP 2FA and get recovery codes | Yes |
| `/2fa/disable` | POST | Disable TOTP 2FA | Yes |
| `/2fa/login-verify` | POST | Verify 2FA code during login | No |
| `/2fa/recovery-codes` | POST | Generate new recovery codes | Yes |
| `/2fa/methods` | GET | Get available 2FA methods for the app | No |
| `/2fa/email/enable` | POST | Enable email-based 2FA | Yes |
| `/2fa/email/resend` | POST | Resend email 2FA code during login | No |
| `/2fa/trusted-devices` | GET | List trusted devices | Yes |
| `/2fa/trusted-devices/:id` | DELETE | Revoke a trusted device | Yes |
| `/2fa/trusted-devices` | DELETE | Revoke all trusted devices | Yes |

### SMS / Phone 2FA

| Endpoint | Method | Description | Auth |
|----------|--------|-------------|------|
| `/phone` | POST | Add/update phone number | Yes |
| `/phone/verify` | POST | Verify phone number with OTP | Yes |
| `/phone` | DELETE | Remove phone number | Yes |
| `/phone/status` | GET | Get phone number status | Yes |
| `/2fa/sms/enable` | POST | Enable SMS-based 2FA | Yes |
| `/2fa/sms/resend` | POST | Resend SMS 2FA code during login | No |

### Backup Email 2FA

| Endpoint | Method | Description | Auth |
|----------|--------|-------------|------|
| `/2fa/backup-email` | POST | Add or update backup email address | Yes |
| `/2fa/backup-email` | DELETE | Remove backup email address | Yes |
| `/2fa/backup-email/status` | GET | Get backup email status | Yes |
| `/2fa/backup-email/enable` | POST | Enable backup email as 2FA method | Yes |
| `/2fa/backup-email/disable` | POST | Disable backup email 2FA | Yes |
| `/2fa/backup-email/resend` | POST | Resend backup email 2FA code during login | No |
| `/2fa/backup-email/verify` | GET | Verify backup email address via link | No |

---

## Passkeys (WebAuthn)

### Registration and Management (Protected)

| Endpoint | Method | Description | Auth |
|----------|--------|-------------|------|
| `/passkey/register/begin` | POST | Start passkey registration ceremony | Yes |
| `/passkey/register/finish` | POST | Complete passkey registration with attestation response | Yes |
| `/passkeys` | GET | List all registered passkeys | Yes |
| `/passkeys/:id` | PUT | Rename a passkey | Yes |
| `/passkeys/:id` | DELETE | Delete a passkey | Yes |

### Passkey as 2FA Method

| Endpoint | Method | Description | Auth |
|----------|--------|-------------|------|
| `/2fa/passkey/begin` | POST | Begin passkey-based 2FA verification during login | No |
| `/2fa/passkey/finish` | POST | Complete passkey 2FA verification and receive JWT tokens | No |

### Passwordless Login

| Endpoint | Method | Description | Auth |
|----------|--------|-------------|------|
| `/passkey/login/begin` | POST | Begin passwordless login (discoverable credential) | No |
| `/passkey/login/finish` | POST | Complete passwordless login and receive JWT tokens | No |

---

## Magic Link Authentication

| Endpoint | Method | Description | Auth |
|----------|--------|-------------|------|
| `/magic-link/request` | POST | Request a magic link login email | No |
| `/magic-link/verify` | POST | Verify magic link token and log in | No |

---

## Social Authentication

| Endpoint | Method | Description | Auth |
|----------|--------|-------------|------|
| `/auth/google/login` | GET | Initiate Google OAuth2 | No |
| `/auth/google/callback` | GET | Google OAuth2 callback | No |
| `/auth/facebook/login` | GET | Initiate Facebook OAuth2 | No |
| `/auth/facebook/callback` | GET | Facebook OAuth2 callback | No |
| `/auth/github/login` | GET | Initiate GitHub OAuth2 | No |
| `/auth/github/callback` | GET | GitHub OAuth2 callback | No |

### Social Account Linking (Protected)

| Endpoint | Method | Description | Auth |
|----------|--------|-------------|------|
| `/profile/social-accounts` | GET | List linked social accounts | Yes |
| `/profile/social-accounts/:id` | DELETE | Unlink a social account | Yes |
| `/auth/google/link` | GET | Initiate Google account linking | Yes |
| `/auth/google/link/callback` | GET | Google link callback | No |
| `/auth/facebook/link` | GET | Initiate Facebook account linking | Yes |
| `/auth/facebook/link/callback` | GET | Facebook link callback | No |
| `/auth/github/link` | GET | Initiate GitHub account linking | Yes |
| `/auth/github/link/callback` | GET | GitHub link callback | No |

---

## Session Management (Protected)

| Endpoint | Method | Description | Auth |
|----------|--------|-------------|------|
| `/sessions` | GET | List all active sessions (devices/IPs) | Yes |
| `/sessions/:id` | DELETE | Revoke a specific session | Yes |
| `/sessions` | DELETE | Revoke all sessions except the current one | Yes |

---

## User Management

| Endpoint | Method | Description | Auth |
|----------|--------|-------------|------|
| `/profile` | GET | Get user profile (includes roles) | Yes |
| `/profile` | PUT | Update user profile | Yes |
| `/profile/email` | PUT | Update user email | Yes |
| `/profile/password` | PUT | Update user password | Yes |
| `/profile` | DELETE | Delete user account | Yes |
| `/auth/validate` | GET | Validate JWT token | Yes |

---

## RBAC Administration (Admin)

| Endpoint | Method | Description | Auth |
|----------|--------|-------------|------|
| `/admin/rbac/roles` | GET | List roles (filtered by app_id) | Admin |
| `/admin/rbac/roles/:id` | GET | Get role by ID with permissions | Admin |
| `/admin/rbac/roles` | POST | Create a new role | Admin |
| `/admin/rbac/roles/:id` | PUT | Update role name/description | Admin |
| `/admin/rbac/roles/:id` | DELETE | Delete a non-system role | Admin |
| `/admin/rbac/roles/:id/permissions` | PUT | Set role permissions | Admin |
| `/admin/rbac/permissions` | GET | List all permissions | Admin |
| `/admin/rbac/permissions` | POST | Create a new permission | Admin |
| `/admin/rbac/user-roles` | GET | List user-role assignments (filtered by app_id) | Admin |
| `/admin/rbac/user-roles` | POST | Assign a role to a user | Admin |
| `/admin/rbac/user-roles` | DELETE | Revoke a role from a user | Admin |
| `/admin/rbac/user-roles/user` | GET | Get roles for a specific user | Admin |

---

## Activity Logs

| Endpoint | Method | Description | Auth |
|----------|--------|-------------|------|
| `/activity-logs` | GET | Get user's activity logs (paginated) | Yes |
| `/activity-logs/:id` | GET | Get specific activity log | Yes |
| `/activity-logs/event-types` | GET | Get available event types | Yes |
| `/activity-logs/export` | GET | Export user's activity logs as CSV | Yes |
| `/admin/activity-logs` | GET | Get all users' logs (admin) | Admin |
| `/admin/activity-logs/export` | GET | Export all activity logs as CSV | Admin |

---

## App Configuration

| Endpoint | Method | Description | Auth |
|----------|--------|-------------|------|
| `/app-config/:app_id` | GET | Get public login configuration for an app (branding, 2FA flags, enabled providers) | No |

---

## OIDC Provider

> **Requires `OIDC_ENABLED=true`** on the application. Routes are mounted under `/oidc/:app_id/`.

| Endpoint | Method | Description | Auth |
|----------|--------|-------------|------|
| `/.well-known/openid-configuration` | GET | Global OIDC discovery document redirect | No |
| `/oidc/:app_id/.well-known/openid-configuration` | GET | OIDC discovery document | No |
| `/oidc/:app_id/.well-known/jwks.json` | GET | JSON Web Key Set (RS256 public key) | No |
| `/oidc/:app_id/authorize` | GET | Authorization endpoint (login UI) | No |
| `/oidc/:app_id/authorize` | POST | Submit authorization form | No |
| `/oidc/:app_id/token` | POST | Token endpoint (code exchange, refresh, client_credentials) | No |
| `/oidc/:app_id/userinfo` | GET | UserInfo endpoint | Bearer token |
| `/oidc/:app_id/userinfo` | POST | UserInfo endpoint | Bearer token |
| `/oidc/:app_id/introspect` | POST | Token introspection | Client credentials |
| `/oidc/:app_id/revoke` | POST | Token revocation | Client credentials |
| `/oidc/:app_id/end_session` | GET | End session (logout) | No |
| `/oidc/:app_id/end_session` | POST | End session (logout) | No |

---

## Authentication Flows

### Standard Authentication

```
1. POST /register or /login    --> Returns JWT access & refresh tokens
2. Include in header:          Authorization: Bearer <access_token>
3. POST /refresh-token         --> Get new tokens when access token expires
4. POST /logout                --> Revoke tokens (blacklisted in Redis)
```

### Two-Factor Authentication

```
1. POST /2fa/generate          --> Get QR code and secret
2. POST /2fa/verify-setup      --> Verify TOTP code from authenticator app
3. POST /2fa/enable            --> Enable 2FA, receive recovery codes
4. POST /login                 --> Returns temporary token (if 2FA enabled)
5. POST /2fa/login-verify      --> Verify TOTP or recovery code --> Get full JWT tokens
```

### Passkey 2FA

```
1. Register a passkey:         POST /passkey/register/begin + /finish
2. POST /login                 --> Returns temporary token (if 2FA enabled with passkey method)
3. POST /2fa/passkey/begin     --> Get assertion options for passkey verification
4. POST /2fa/passkey/finish    --> Verify passkey assertion --> Get full JWT tokens
```

### Passwordless Login (Passkey)

```
1. Register a passkey:         POST /passkey/register/begin + /finish (one-time setup)
2. POST /passkey/login/begin   --> Get assertion options for discoverable credentials
3. POST /passkey/login/finish  --> Verify passkey assertion --> Get full JWT tokens
```

### Magic Link Authentication

```
1. POST /magic-link/request    --> Send magic link email to user
2. User clicks link in email
3. POST /magic-link/verify     --> Verify token from email --> Get full JWT tokens
```

### Social Authentication

```
1. GET /auth/{provider}/login  --> Redirect to provider (Google, Facebook, GitHub)
2. User authorizes on provider's site
3. GET /auth/{provider}/callback --> Provider redirects back
4. Receive JWT tokens for authenticated user
```

### Social Account Linking

```
1. Authenticate normally       --> Get JWT tokens
2. GET /auth/{provider}/link   --> Redirect to provider for linking (requires auth)
3. User authorizes on provider's site
4. GET /auth/{provider}/link/callback --> Provider redirects back
5. Social account is linked to existing user
```

### SMS 2FA

```
1. POST /phone                 --> Register phone number
2. POST /phone/verify          --> Verify phone with OTP
3. POST /2fa/sms/enable        --> Enable SMS 2FA
4. POST /login                 --> Returns temporary token (if 2FA enabled)
5. POST /2fa/login-verify      --> Submit SMS OTP code --> Get full JWT tokens
   (or POST /2fa/sms/resend   --> Resend SMS code if not received)
```

### OIDC Authorization Code Flow

```
1. GET /oidc/:app_id/authorize --> Redirect to hosted login UI
2. User authenticates (credentials, social, passkey, magic link)
3. POST /oidc/:app_id/authorize --> Submit credentials, receive redirect with ?code=
4. POST /oidc/:app_id/token     --> Exchange code for access_token + id_token + refresh_token
5. GET  /oidc/:app_id/userinfo  --> Fetch user claims with access token
6. POST /oidc/:app_id/end_session --> Logout and revoke session
```
