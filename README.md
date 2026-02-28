<div align="center">

# Authentication API

### Production-Ready Go REST API for Authentication

A complete authentication and authorization system with multi-tenancy, social login, two-factor authentication, email verification, JWT tokens, admin GUI, and activity logging.

[![Go Version](https://img.shields.io/badge/Go-1.23+-00ADD8?style=flat&logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![Docker](https://img.shields.io/badge/Docker-Supported-2496ED?style=flat&logo=docker)](https://www.docker.com/)
[![Swagger](https://img.shields.io/badge/API-Swagger-85EA2D?style=flat&logo=swagger)](http://localhost:8080/swagger/index.html)

[Quick Start](#quick-start) · [Documentation](#documentation) · [Contributing](#contributing)

</div>

---

## Features

- **Multi-Tenancy** -- Serve multiple organizations and applications from a single deployment with complete data isolation
- **Authentication** -- Registration, login, JWT access/refresh tokens, token blacklisting, password reset, email verification
- **Two-Factor Authentication** -- TOTP with authenticator apps and recovery codes
- **Social Login** -- Google, Facebook, and GitHub OAuth2
- **Admin GUI** -- Built-in web panel for managing tenants, apps, users, OAuth configs, API keys, and settings
- **Activity Logging** -- Smart event categorization, anomaly detection, and automatic retention cleanup
- **Security Hardening** -- Rate limiting, security headers, timing-safe CSRF, JWT token type enforcement
- **API Documentation** -- Interactive Swagger UI

---

## Quick Start

**Prerequisites:** Docker & Docker Compose (recommended), or Go 1.23+, PostgreSQL 13+, Redis 6+

```bash
# Clone and configure
git clone <repository-url>
cd <project-directory>
cp .env.example .env        # Edit with your settings

# Start services
./setup-network.sh create   # First time only
make docker-dev              # Start PostgreSQL, Redis, and the API
make migrate-up              # Apply database migrations
```

The API is now running at `http://localhost:8080`
Swagger docs at `http://localhost:8080/swagger/index.html`

All API requests require the `X-App-ID` header. The default app ID `00000000-0000-0000-0000-000000000001` is created automatically.

```bash
curl -X POST http://localhost:8080/auth/register \
  -H "X-App-ID: 00000000-0000-0000-0000-000000000001" \
  -H "Content-Type: application/json" \
  -d '{"email":"user@example.com","password":"Pass123!@#"}'
```

For detailed setup instructions, see [Getting Started](docs/getting-started.md).

---

## Documentation

| Document | Description |
|----------|-------------|
| **[Getting Started](docs/getting-started.md)** | Installation, setup, and first steps |
| **[Configuration](docs/configuration.md)** | Environment variables and OAuth setup |
| **[API Endpoints](docs/api-endpoints.md)** | Full endpoint reference and auth flows |
| **[Multi-Tenancy](docs/multi-tenancy.md)** | Tenant/app management and data isolation |
| **[Admin GUI](docs/admin-gui.md)** | Built-in admin panel setup and usage |
| **[Activity Logging](docs/activity-logging.md)** | Smart logging, anomaly detection, retention |
| **[Database Migrations](docs/database-migrations.md)** | Migration system and commands |
| **[Testing](docs/testing.md)** | Running tests and coverage |
| **[Project Structure](docs/project-structure.md)** | Codebase layout and architecture |
| **[Makefile Reference](docs/makefile-reference.md)** | All available make commands |
| **[Architecture](docs/ARCHITECTURE.md)** | System design and patterns |
| **[API Reference (detailed)](docs/API.md)** | Full request/response documentation |
| **[Changelog](CHANGELOG.md)** | Version history and release notes |

For early fork users upgrading from before multi-tenancy was added, see the [Pre-Release Migration Reference](docs/BREAKING_CHANGES.md).

---

## Tech Stack

| Category | Technology |
|----------|-----------|
| Language | Go 1.23+ |
| Web Framework | [Gin](https://github.com/gin-gonic/gin) |
| Database | PostgreSQL 13+ with [GORM](https://gorm.io/) |
| Cache/Sessions | Redis 6+ with [go-redis](https://github.com/redis/go-redis) |
| Authentication | JWT ([golang-jwt](https://github.com/golang-jwt/jwt)), OAuth2 |
| 2FA | TOTP ([pquerna/otp](https://github.com/pquerna/otp)) |
| API Docs | [Swagger/Swaggo](https://github.com/swaggo/swag) |
| Admin GUI | Go Templates, HTMX, Bootstrap 5 |
| Containerization | Docker, Docker Compose |

---

## Contributing

Contributions are welcome. Please read [CONTRIBUTING.md](CONTRIBUTING.md) and [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md) before opening a pull request.

```bash
# Development workflow
make dev              # Start with hot reload
make test             # Run tests
make fmt && make lint # Format and lint
make security         # Security checks
```

---

## Security

For reporting vulnerabilities, **do not create public issues**. Read [SECURITY.md](SECURITY.md) for instructions on responsible disclosure.

---

## License

This project is licensed under the MIT License. See [LICENSE](LICENSE) for details.
