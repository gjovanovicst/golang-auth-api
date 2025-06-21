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
- `internal/user` — User management
- `internal/social` — Social login
- `internal/email` — Email verification/reset
- `internal/middleware` — JWT, RBAC, etc.
- `pkg/jwt` — JWT helpers

## Flow Example
1. User registers or logs in
2. API validates credentials, issues JWT
3. JWT used for protected endpoints
4. Redis manages sessions/tokens
5. Social login handled via OAuth2 callback

---
For more details, see the code and comments in each package.
