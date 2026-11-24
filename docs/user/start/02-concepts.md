# Concepts

Five ideas explain most of the API surface. Read this once and the endpoint reference
becomes obvious.

## Tenants and applications

```
Tenant "Acme"
 ├── Application "Acme Web"     app_id = 0000…0001
 ├── Application "Acme Mobile"  app_id = 0000…0002
 └── Application "Acme Admin"   app_id = 0000…0003
```

- A **tenant** is an organisation. It owns applications and is the unit of billing or
  ownership in your own product.
- An **application** is a client of the Auth API: a website, a mobile app or a service.
  It has its own users, roles, branding, password policy, 2FA policy, token TTLs,
  OAuth credentials and email templates.

**Users are scoped to an application.** The same email registered in two applications
produces two different users, two different password hashes and two different sets of
roles. There is no shared identity unless you create a [session group](12-sessions-sso.md).

## The `X-App-ID` header

Every request to a public or protected endpoint must identify the application:

```bash
curl http://localhost:8080/profile \
  -H "X-App-ID: 00000000-0000-0000-0000-000000000001" \
  -H "Authorization: Bearer $ACCESS_TOKEN"
```

Exceptions: `/swagger/*`, `/admin/*` (admin key instead), `/gui/*` (GUI session) and
`/oidc/:app_id/*` (app id is in the path). Social callbacks read the app id from the
signed `state` parameter.

## Credentials and tokens

| Credential | Used by | Sent as |
|---|---|---|
| Access token (JWT) | end users | `Authorization: Bearer …` |
| Refresh token | end users | body of `POST /refresh-token` and `POST /logout` |
| Temporary 2FA token | login flow | body of `/2fa/login-verify` (10-minute TTL) |
| Admin API key | automation against `/admin/*` | `X-Admin-API-Key: …` |
| Per-app API key | automation against `/app/:id/*` | `X-App-ID` + `X-App-API-Key: …` |
| Admin session cookie | the admin GUI | `Set-Cookie` from `/gui/login` |

Access tokens are short-lived (15 minutes by default) and are checked against a Redis
blacklist on logout or password change. Refresh tokens rotate: each refresh returns a
new pair and invalidates the previous refresh token.

## Sessions

Every login creates a **session** in Redis: an id, the user, the app, IP address, user
agent and the current refresh token. Sessions are what the GUI shows on the Sessions
page, what `GET /sessions` returns to the user, and what session groups share across
applications.

Revoking a session deletes the refresh token — the access token expires naturally
within its TTL unless the user was also blacklisted.

## Roles and permissions

Authorization is RBAC, scoped per application:

- **Permissions** are global (`user:read`, `user:write`, `settings:write`, `log:read`, …).
- **Roles** belong to an application (`admin`, `member`, `viewer` by default).
- **User roles** attach a user to a role inside one application.

Protected endpoints declare a required permission; for example `GET /profile` requires
`user:read` and `PUT /profile` requires `user:write`.

## Settings precedence

Behaviour can be configured globally (environment) and per application (database). The
resolution order is:

```
environment variable  →  application setting  →  built-in default
```

This applies to token TTLs, password policy, 2FA requirements, branding, CORS and
brute-force protection. The admin GUI edits the application layer; changing an
environment variable therefore overrides the GUI value.

## Next

- [Authentication](10-authentication.md) — registration, login, refresh and logout.
- [Two-factor and recovery](11-2fa-recovery.md) — every second factor and how to recover.
