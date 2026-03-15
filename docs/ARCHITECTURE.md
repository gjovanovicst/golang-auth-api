# Architecture Overview

## High-Level Diagram

```
+-----------+      +-----------+      +-----------+
|  Client   |<---> |  API      |<---> | Database  |
| (Frontend)|      | (Gin)     |      | (Postgres)|
+-----------+      +-----------+      +-----------+
                         |
                         v
                   +-----------+
                   |  Redis    |
                   +-----------+
```

- **API**: Go (Gin), handles authentication, authorization, and business logic
- **Database**: PostgreSQL, stores users, tokens, etc.
- **Redis**: Session/token management, caching, rate limiting
- **Email**: SMTP for verification and password reset
- **Social Auth**: OAuth2 with Google, Facebook, GitHub
- **OIDC**: Built-in OpenID Connect provider (RS256, PKCE, JWKS) — per application, opt-in
- **Webhooks**: Async HMAC-signed event delivery with retry queue
- **GeoIP**: MaxMind GeoLite2 for IP access rules (CIDR/country per application)
- **SMS**: Twilio integration for SMS-based 2FA
- **Metrics**: Prometheus endpoint for system and request observability

## Key Components
- `internal/auth` — Core authentication logic
- `internal/user` — User management, magic link login, resend verification, import/export
- `internal/social` — Social login + social account linking/unlinking
- `internal/email` — Email verification/reset
- `internal/webauthn` — WebAuthn/passkey registration, 2FA, and passwordless login
- `internal/rbac` — Role-based access control (roles, permissions, user-role assignments)
- `internal/session` — Session management (list/revoke active sessions)
- `internal/twofa` — 2FA: TOTP, email, SMS, backup email, trusted devices, recovery codes
- `internal/oidc` — OIDC provider (discovery, authorize, token, userinfo, introspect, revoke, end_session, JWKS)
- `internal/webhook` — Webhook endpoint registry + async delivery dispatcher with retries
- `internal/bruteforce` — Account lockout, progressive delays, CAPTCHA threshold
- `internal/geoip` — MaxMind GeoLite2 lookup + IP rule evaluation (CIDR/country per app)
- `internal/health` — `GET /health` liveness check, `GET /metrics` Prometheus, `PrometheusMiddleware`
- `internal/sms` — SMS sender interface + Twilio implementation
- `internal/middleware` — JWT, RBAC, rate limiting, session validation (Redis)
- `internal/admin` — Admin API + Admin GUI (HTMX-based)
- `pkg/jwt` — JWT helpers (tokens include `roles` claim)

## Flow Example
1. User registers or logs in (email/password, social, passkey, or magic link)
2. API validates credentials, issues JWT (includes `roles` claim)
3. JWT used for protected endpoints; session validated against Redis on every request
4. Redis manages sessions/tokens
5. Social login handled via OAuth2 callback; accounts can be linked/unlinked
6. Passkey 2FA or passwordless login via WebAuthn (FIDO2)
7. Magic link login sends a one-time link via email
8. RBAC enforces per-application role and permission checks
9. Brute-force protection applies lockout and progressive delays on failed logins
10. GeoIP evaluates IP access rules (CIDR/country allow/block lists) per application
11. Webhooks fire async HMAC-signed POST requests on auth events
12. OIDC relying-party clients can use each application as an OAuth2/OIDC issuer

---
For more details, see the code and comments in each package.
