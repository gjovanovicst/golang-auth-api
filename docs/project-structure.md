# Project Structure

```
project-root/
├── cmd/
│   ├── api/                    # Application entry point
│   │   └── main.go
│   ├── migrate_oauth/          # OAuth migration tool
│   │   └── main.go
│   └── setup/                  # Admin account setup wizard
│       └── main.go
├── internal/                   # Private application code
│   ├── admin/                  # Admin API (tenant/app/OAuth management) + Admin GUI
│   ├── auth/                   # Authentication handlers
│   ├── user/                   # User management (includes magic link login, import/export)
│   ├── social/                 # Social OAuth2 providers + social account linking
│   ├── twofa/                  # Two-factor authentication (TOTP, email, SMS, backup email, trusted devices)
│   ├── webauthn/               # WebAuthn/passkey registration, 2FA, and passwordless login
│   ├── rbac/                   # Role-based access control (roles, permissions, user-roles)
│   ├── session/                # Session management (list/revoke active sessions)
│   ├── oidc/                   # OIDC provider (Authorization Code + PKCE, RS256 ID tokens, JWKS)
│   ├── webhook/                # Webhook system (endpoint registry, async delivery queue, retries)
│   ├── bruteforce/             # Brute-force protection (account lockout, progressive delays, CAPTCHA)
│   ├── geoip/                  # GeoIP service (MaxMind) + IP access rules (CIDR/country per app)
│   ├── health/                 # Health check + Prometheus metrics endpoint
│   ├── sms/                    # SMS sender interface + Twilio implementation
│   ├── log/                    # Activity logging system
│   ├── email/                  # Email verification & reset
│   ├── middleware/             # JWT auth, AppID, CORS, rate limiting, security headers, session validation
│   ├── database/               # Database connection & migrations
│   ├── redis/                  # Redis connection & operations
│   ├── config/                 # Configuration management
│   └── util/                   # Utility functions
├── pkg/                        # Public packages
│   ├── models/                 # Database models (GORM) — includes WebAuthn credentials, roles, permissions
│   ├── dto/                    # Data transfer objects — includes WebAuthn, RBAC, session, magic link DTOs
│   ├── errors/                 # Custom error types
│   └── jwt/                    # JWT token utilities
├── web/                        # Shared web context keys and interfaces
├── docs/                       # Documentation
│   ├── features/               # Feature-specific docs
│   ├── guides/                 # Setup and configuration guides
│   ├── migrations/             # Migration system docs
│   ├── implementation/         # Implementation details
│   └── implementation_phases/  # Original project phases
├── migrations/                 # SQL migration files
├── scripts/                    # Helper scripts (migrate, backup, cleanup)
├── .github/                    # GitHub configuration (CI/CD, issue templates)
├── Dockerfile                  # Production Docker image
├── Dockerfile.dev              # Development Docker image
├── docker-compose.yml          # Production compose config
├── docker-compose.dev.yml      # Development compose config
├── Makefile                    # Build and development commands
├── .air.toml                   # Hot reload configuration
├── .env.example                # Environment variables template
├── go.mod / go.sum             # Go module dependencies
├── CONTRIBUTING.md             # Contribution guidelines
├── CODE_OF_CONDUCT.md          # Code of conduct
├── SECURITY.md                 # Security policy
├── CHANGELOG.md                # Version history
└── LICENSE                     # MIT License
```

---

## Key Files

| File | Purpose |
|------|---------|
| `cmd/api/main.go` | Entry point -- dependency injection and route setup |
| `pkg/models/` | GORM database models |
| `pkg/models/webauthn_credential.go` | WebAuthn/passkey credential model |
| `pkg/models/role.go` | Role, Permission, and UserRole models |
| `pkg/models/oidc_client.go` | OIDC relying-party client model |
| `pkg/models/oidc_auth_code.go` | OIDC authorization code model |
| `pkg/models/webhook_endpoint.go` | Webhook endpoint model |
| `pkg/models/webhook_delivery.go` | Webhook delivery history model |
| `pkg/models/ip_rule.go` | IP access rule model (CIDR/country) |
| `pkg/models/api_key_usage.go` | Per-API-key daily usage analytics model |
| `pkg/models/trusted_device.go` | Trusted device (2FA bypass) model |
| `pkg/dto/` | API request/response data transfer objects |
| `pkg/dto/webauthn.go` | Passkey registration/login DTOs |
| `pkg/dto/rbac.go` | RBAC DTOs (roles, permissions, user-roles) |
| `pkg/dto/session.go` | Session management DTOs |
| `pkg/errors/errors.go` | Custom error types |
| `pkg/jwt/jwt.go` | JWT token creation and validation |
| `.env.example` | Environment variable template |

---

## Architecture

Each domain follows the **Repository-Service-Handler** pattern:

- **Repository** -- Data access and database queries
- **Service** -- Business logic, validation, orchestration
- **Handler** -- HTTP transport, request binding, response formatting

For the full architecture documentation, see [ARCHITECTURE.md](ARCHITECTURE.md).
