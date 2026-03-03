# API Endpoints

All endpoints (except `/swagger/*`, `/admin/*`, and OAuth callbacks) require the `X-App-ID` header.

Interactive documentation is available at [http://localhost:8080/swagger/index.html](http://localhost:8080/swagger/index.html) when the server is running.

For detailed request/response schemas, see [API.md](API.md).

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
| `/2fa/generate` | POST | Generate 2FA secret and QR code | Yes |
| `/2fa/verify-setup` | POST | Verify initial 2FA setup | Yes |
| `/2fa/enable` | POST | Enable 2FA and get recovery codes | Yes |
| `/2fa/disable` | POST | Disable 2FA | Yes |
| `/2fa/login-verify` | POST | Verify 2FA code during login | No |
| `/2fa/recovery-codes` | POST | Generate new recovery codes | Yes |

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
| `/admin/activity-logs` | GET | Get all users' logs (admin) | Yes |

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
