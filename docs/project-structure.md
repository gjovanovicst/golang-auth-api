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
│   ├── user/                   # User management (includes magic link login)
│   ├── social/                 # Social OAuth2 providers + social account linking
│   ├── twofa/                  # Two-factor authentication
│   ├── webauthn/               # WebAuthn/passkey registration, 2FA, and passwordless login
│   ├── rbac/                   # Role-based access control (roles, permissions, user-roles)
│   ├── session/                # Session management (list/revoke active sessions)
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
