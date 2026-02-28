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

## Social Authentication

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/auth/google/login` | GET | Initiate Google OAuth2 |
| `/auth/google/callback` | GET | Google OAuth2 callback |
| `/auth/facebook/login` | GET | Initiate Facebook OAuth2 |
| `/auth/facebook/callback` | GET | Facebook OAuth2 callback |
| `/auth/github/login` | GET | Initiate GitHub OAuth2 |
| `/auth/github/callback` | GET | GitHub OAuth2 callback |

---

## User Management

| Endpoint | Method | Description | Auth |
|----------|--------|-------------|------|
| `/profile` | GET | Get user profile | Yes |
| `/auth/validate` | GET | Validate JWT token | Yes |

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

### Social Authentication

```
1. GET /auth/{provider}/login  --> Redirect to provider (Google, Facebook, GitHub)
2. User authorizes on provider's site
3. GET /auth/{provider}/callback --> Provider redirects back
4. Receive JWT tokens for authenticated user
```
