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
- **Redis**: Session/token management, caching
- **Email**: SMTP for verification and password reset
- **Social Auth**: OAuth2 with Google, Facebook, GitHub

## Key Components
- `internal/auth` — Core authentication logic
- `internal/user` — User management, magic link login, resend verification
- `internal/social` — Social login + social account linking/unlinking
- `internal/email` — Email verification/reset
- `internal/webauthn` — WebAuthn/passkey registration, 2FA, and passwordless login
- `internal/rbac` — Role-based access control (roles, permissions, user-role assignments)
- `internal/session` — Session management (list/revoke active sessions)
- `internal/middleware` — JWT, RBAC, rate limiting, session validation (Redis)
- `internal/admin` — Admin API + Admin GUI (HTMX-based)
- `pkg/jwt` — JWT helpers (tokens now include `roles` claim)

## Flow Example
1. User registers or logs in (email/password, social, passkey, or magic link)
2. API validates credentials, issues JWT (includes `roles` claim)
3. JWT used for protected endpoints; session validated against Redis on every request
4. Redis manages sessions/tokens
5. Social login handled via OAuth2 callback; accounts can be linked/unlinked
6. Passkey 2FA or passwordless login via WebAuthn (FIDO2)
7. Magic link login sends a one-time link via email
8. RBAC enforces per-application role and permission checks

---
For more details, see the code and comments in each package.
