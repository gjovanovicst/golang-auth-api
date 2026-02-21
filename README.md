<div align="center">

# 🔐 Authentication API

### Modern, Production-Ready Go REST API with Multi-Tenancy

A comprehensive authentication and authorization system with multi-tenancy support, social login, email verification, JWT, Two-Factor Authentication, and smart activity logging.

[![Go Version](https://img.shields.io/badge/Go-1.23+-00ADD8?style=flat&logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![Docker](https://img.shields.io/badge/Docker-Supported-2496ED?style=flat&logo=docker)](https://www.docker.com/)
[![Swagger](https://img.shields.io/badge/API-Swagger-85EA2D?style=flat&logo=swagger)](http://localhost:8080/swagger/index.html)

[Features](#-features) • [Quick Start](#-quick-start) • [Admin GUI](#-admin-gui-v30) • [Multi-Tenancy](#-multi-tenancy-v20) • [Documentation](#-documentation) • [API Endpoints](#-api-endpoints) • [Contributing](#-contributing)

</div>

---

## ✨ Features

### 🏢 Multi-Tenancy Architecture (v2.0+)
- ✅ **Multi-Tenant Support** - Serve multiple organizations from single deployment
- ✅ **Application Management** - Multiple apps per tenant with data isolation
- ✅ **Per-App OAuth Configuration** - Database-backed OAuth credentials
- ✅ **Admin API** - Manage tenants, applications, and OAuth providers
- ✅ **Complete Data Isolation** - Tenant/app separation at database level

### 🖥️ Admin GUI (v3.0+)
- ✅ **Built-In Admin Panel** - Web-based GUI served at `/gui/*` from the same binary
- ✅ **Dashboard** - Overview with tenant, app, user, and log counts
- ✅ **Tenant & App Management** - Full CRUD with HTMX-powered single-page interactions
- ✅ **OAuth Config Management** - Per-app provider setup with inline toggle and secret masking
- ✅ **User Management** - Search, filter, view details, and toggle user active status
- ✅ **Activity Log Viewer** - Multi-filter log viewer with inline detail panel
- ✅ **API Key Management** - Admin and per-app API keys with SHA-256 hashed storage
- ✅ **System Settings** - Accordion-based settings page with per-setting inline save/reset
- ✅ **Embedded Assets** - Bootstrap 5, HTMX, and Bootstrap Icons embedded via `go:embed`

### 🛡️ Security Hardening (v3.0+)
- ✅ **Rate Limiting** - Configurable per-route rate limiting with Redis + in-memory fallback
- ✅ **Security Headers** - CSP, HSTS, X-Frame-Options, and more on every response
- ✅ **JWT Token Type Enforcement** - Prevents refresh tokens from being used as access tokens
- ✅ **Timing-Safe CSRF** - Constant-time comparison for CSRF token validation
- ✅ **Password Max Length** - bcrypt DoS prevention with 128-char limit on all password fields

### 🔑 Authentication & Authorization
- ✅ **Secure Registration & Login** with JWT access/refresh tokens
- ✅ **Two-Factor Authentication (2FA)** with TOTP and recovery codes
- ✅ **Social Authentication** (Google, Facebook, GitHub OAuth2)
- ✅ **Email Verification** and password reset flows
- ✅ **Token Blacklisting** for secure logout
- ✅ **Role-Based Access Control** with middleware

### 📊 Smart Activity Logging
- ✅ **Intelligent Event Categorization** (Critical/Important/Informational)
- ✅ **Anomaly Detection** (new IP address, device detection)
- ✅ **Automatic Log Retention** and cleanup (80-95% database size reduction)
- ✅ **Pagination & Filtering** for audit trails
- ✅ **Configurable Logging** via environment variables

### 🛠️ Developer Experience
- ✅ **Interactive Swagger Documentation** at `/swagger/index.html`
- ✅ **Docker & Docker Compose** for easy setup
- ✅ **Hot Reload** development with Air
- ✅ **Database Migrations** with tracking system
- ✅ **Comprehensive Testing** suite
- ✅ **CI/CD with GitHub Actions** (test, build, security scan)
- ✅ **Local CI Testing** with `act` support
- ✅ **Professional Project Structure**

### 🚀 Production Ready
- ✅ **Redis Integration** for caching and session management
- ✅ **PostgreSQL Database** with GORM ORM
- ✅ **Security Best Practices** (OWASP guidelines)
- ✅ **Automated Cleanup Jobs**
- ✅ **Environment-Based Configuration**

---

## 🚀 Quick Start

### Prerequisites
- **Docker & Docker Compose** (recommended)
- Or: Go 1.23+, PostgreSQL 13+, Redis 6+

### Installation

```bash
# 1. Clone the repository
git clone <repository-url>
cd <project-directory>

# 2. Copy environment configuration
cp .env.example .env
# Edit .env with your configuration

# 3. Create shared network (Required for first start)
./setup-network.sh create

# 4. Start with Docker (recommended)
make docker-dev
# Or: Windows: dev.bat | Linux/Mac: ./dev.sh

# 5. Apply database migrations (includes multi-tenancy)
make migrate-up

# 6. (Optional) Migrate OAuth credentials to database
go run cmd/migrate_oauth/main.go
```

**🎉 That's it!** Your API is now running at `http://localhost:8080`

### What Just Happened?
- ✅ PostgreSQL & Redis started in Docker containers
- ✅ Database tables created (multi-tenant architecture)
- ✅ Default tenant and application created (`00000000-0000-0000-0000-000000000001`)
- ✅ Application running with hot reload enabled
- ✅ Swagger docs available at [http://localhost:8080/swagger/index.html](http://localhost:8080/swagger/index.html)

### ⚠️ Important: Multi-Tenancy (v2.0+)

**All API requests require the `X-App-ID` header:**

```bash
curl -X POST http://localhost:8080/auth/register \
  -H "X-App-ID: 00000000-0000-0000-0000-000000000001" \
  -H "Content-Type: application/json" \
  -d '{"email":"user@example.com","password":"Pass123!@#"}'
```

**Default Application ID:** `00000000-0000-0000-0000-000000000001` (created automatically)

**Upgrading from v1.x?** See [Migration Guide](#-upgrading-from-v1x-to-v20) below.

### Next Steps
- 📖 [Configure Environment Variables](#-environment-configuration)
- 🔧 [Set up Social OAuth Providers](#social-authentication-setup)
- 🏢 [Multi-Tenancy Guide](#-multi-tenancy-v20) - Create tenants and apps
- 🖥️ [Admin GUI Setup](#-admin-gui-v30) - Set up the admin dashboard
- 📊 [Configure Activity Logging](docs/features/QUICK_SETUP_LOGGING.md)
- 🗄️ [Learn About Database Migrations](docs/migrations/README.md)

---

## 🖥️ Admin GUI (v3.0+)

The Admin GUI is a built-in web panel for managing your Auth API. It is served from the same binary at `/gui/*` and requires no separate frontend deployment.

### Initial Setup

Create the admin account using the interactive CLI wizard:

```bash
go run cmd/setup/main.go
```

You will be prompted for a username and password (masked input). The account is stored with bcrypt-hashed password in the database.

### Accessing the GUI

Once the server is running, navigate to:

```
http://localhost:8080/gui/login
```

Log in with the credentials you created during setup.

### Features

| Page | Description |
|------|-------------|
| **Dashboard** | Overview of tenants, apps, users, and recent activity |
| **Tenants** | Create, edit, delete tenant organizations |
| **Applications** | Manage apps per tenant with flat list and tenant filter |
| **OAuth Configs** | Configure OAuth providers per-app with inline toggle |
| **Users** | Search users, view details, toggle active/inactive |
| **Activity Logs** | View and filter activity logs with inline detail |
| **API Keys** | Manage admin and per-app API keys |
| **Settings** | View and override system settings |

### Technology Stack

- **Go Templates** with layout/partial composition
- **HTMX** for single-page interactions without full page reloads
- **Bootstrap 5** for responsive UI
- **Bootstrap Icons** for iconography
- All assets embedded via `go:embed` — no external CDN dependencies

---

## 📚 Documentation

### Quick Links
| Document | Description |
|----------|-------------|
| 📖 **[Documentation Index](docs/README.md)** | Complete documentation overview |
| 🏗️ **[Architecture](docs/ARCHITECTURE.md)** | System architecture and design |
| 📡 **[API Reference](docs/API.md)** | Detailed API documentation |
| 🔄 **[Migration Guide](docs/migrations/README.md)** | Database migration system |
| 🚨 **[Breaking Changes](BREAKING_CHANGES.md)** | v2.0 Multi-tenancy breaking changes |
| 📋 **[Changelog](CHANGELOG.md)** | Version history and release notes |
| 🤝 **[Contributing](CONTRIBUTING.md)** | Contribution guidelines |
| 🛡️ **[Security Policy](SECURITY.md)** | Security and vulnerability reporting |

### Documentation by Category

#### 🎯 Getting Started
- [Quick Start Guide](#-quick-start) (above)
- [Environment Variables](docs/guides/ENV_VARIABLES.md)
- [Docker Setup](docs/migrations/MIGRATIONS_DOCKER.md)

#### 📦 Features
- [Activity Logging Guide](docs/features/ACTIVITY_LOGGING_GUIDE.md)
- [Social Login Setup](docs/features/SOCIAL_LOGIN_DATA_STORAGE.md)
- [Profile Management](docs/features/PROFILE_SYNC_ON_LOGIN.md)
- [Security Features](docs/features/SECURITY_TOKEN_BLACKLISTING.md)

#### 🗄️ Database & Migrations
- [Migration System Overview](docs/migrations/MIGRATIONS.md)
- [User Migration Guide](docs/migrations/USER_GUIDE.md)
- [Upgrade Guide](docs/migrations/UPGRADE_GUIDE.md)
- [Breaking Changes](BREAKING_CHANGES.md)
- [Migration Quick Reference](docs/migrations/MIGRATION_QUICK_REFERENCE.md)

#### 🔧 Development
- [Architecture Documentation](docs/ARCHITECTURE.md)
- [Implementation Phases](docs/implementation_phases/README.md)
- [Database Implementation](docs/implementation/DATABASE_IMPLEMENTATION.md)

---

## 🌐 API Endpoints

**⚠️ Important:** All endpoints (except `/swagger/*`, `/admin/*`, and OAuth callbacks) require the `X-App-ID` header.

### Admin API (Multi-Tenancy Management)
| Endpoint | Method | Description | Protected |
|----------|--------|-------------|-----------|
| `/admin/tenants` | POST | Create new tenant | 🔐 Admin |
| `/admin/tenants` | GET | List all tenants (paginated) | 🔐 Admin |
| `/admin/apps` | POST | Create application for tenant | 🔐 Admin |
| `/admin/apps` | GET | List applications (paginated) | 🔐 Admin |
| `/admin/oauth-providers` | POST | Configure OAuth provider for app | 🔐 Admin |
| `/admin/oauth-providers/:app_id` | GET | List OAuth providers for app | 🔐 Admin |
| `/admin/oauth-providers/:id` | PUT | Update OAuth provider config | 🔐 Admin |
| `/admin/oauth-providers/:id` | DELETE | Delete OAuth provider config | 🔐 Admin |

### Authentication
| Endpoint | Method | Description | Protected |
|----------|--------|-------------|-----------|
| `/register` | POST | User registration | ❌ |
| `/login` | POST | User login (with 2FA support) | ❌ |
| `/logout` | POST | Logout and token revocation | ✅ |
| `/refresh-token` | POST | Refresh JWT tokens | ❌ |
| `/verify-email` | GET | Email verification | ❌ |
| `/forgot-password` | POST | Request password reset | ❌ |
| `/reset-password` | POST | Reset password with token | ❌ |

### Two-Factor Authentication (2FA)
| Endpoint | Method | Description | Protected |
|----------|--------|-------------|-----------|
| `/2fa/generate` | POST | Generate 2FA secret and QR code | ✅ |
| `/2fa/verify-setup` | POST | Verify initial 2FA setup | ✅ |
| `/2fa/enable` | POST | Enable 2FA and get recovery codes | ✅ |
| `/2fa/disable` | POST | Disable 2FA | ✅ |
| `/2fa/login-verify` | POST | Verify 2FA code during login | ❌ |
| `/2fa/recovery-codes` | POST | Generate new recovery codes | ✅ |

### Social Authentication
| Endpoint | Method | Description |
|----------|--------|-------------|
| `/auth/google/login` | GET | Initiate Google OAuth2 |
| `/auth/google/callback` | GET | Google OAuth2 callback |
| `/auth/facebook/login` | GET | Initiate Facebook OAuth2 |
| `/auth/facebook/callback` | GET | Facebook OAuth2 callback |
| `/auth/github/login` | GET | Initiate GitHub OAuth2 |
| `/auth/github/callback` | GET | GitHub OAuth2 callback |

### User Management
| Endpoint | Method | Description | Protected |
|----------|--------|-------------|-----------|
| `/profile` | GET | Get user profile | ✅ |
| `/auth/validate` | GET | Validate JWT token | ✅ |

### Activity Logs
| Endpoint | Method | Description | Protected |
|----------|--------|-------------|-----------|
| `/activity-logs` | GET | Get user's activity logs (paginated) | ✅ |
| `/activity-logs/:id` | GET | Get specific activity log | ✅ |
| `/activity-logs/event-types` | GET | Get available event types | ✅ |
| `/admin/activity-logs` | GET | Get all users' logs (admin) | ✅ |

**📖 Full API Documentation:** [Swagger UI](http://localhost:8080/swagger/index.html) (when running)

---

## 🔐 Authentication Flow

### Standard Authentication
```
1. POST /register or /login → Returns JWT access & refresh tokens
2. Include token in header: Authorization: Bearer <token>
3. POST /refresh-token → Get new tokens when expired
4. POST /logout → Revoke tokens and blacklist
```

### Two-Factor Authentication
```
1. POST /2fa/generate → Get QR code and secret
2. POST /2fa/verify-setup → Verify TOTP code
3. POST /2fa/enable → Enable 2FA, receive recovery codes
4. POST /login → Returns temporary token (if 2FA enabled)
5. POST /2fa/login-verify → Verify TOTP/recovery code → Get full JWT tokens
```

### Social Authentication
```
1. GET /auth/{provider}/login → Redirect to provider
2. User authorizes on provider's site
3. GET /auth/{provider}/callback → Provider redirects back
4. Receive JWT tokens for authenticated user
```

---

## 🏢 Multi-Tenancy (v2.0+)

### Overview
The API supports **multi-tenancy**, allowing you to serve multiple organizations (tenants) and applications from a single deployment. Each application has isolated users, OAuth configurations, and activity logs.

### Hierarchy
```
Tenant (Organization)
 └── Application (Mobile App, Web App, etc.)
      ├── Users (isolated per app)
      ├── OAuth Providers (per-app credentials)
      └── Activity Logs (per-app audit trail)
```

### Default Setup
On first installation, a default tenant and application are created:
- **Default Tenant ID:** `00000000-0000-0000-0000-000000000001`
- **Default Application ID:** `00000000-0000-0000-0000-000000000001`
- All existing data (if upgrading from v1.x) is automatically migrated to this default app

### Required Header
All API requests must include the `X-App-ID` header:

```bash
# Example: Register a user
curl -X POST http://localhost:8080/auth/register \
  -H "X-App-ID: 00000000-0000-0000-0000-000000000001" \
  -H "Content-Type: application/json" \
  -d '{
    "email": "user@example.com",
    "password": "SecurePass123!@#"
  }'
```

**Exceptions (no header required):**
- `/swagger/*` - Swagger documentation
- `/admin/*` - Admin API endpoints
- OAuth callbacks (app_id in state parameter)

### Creating Tenants & Applications

#### 1. Create a Tenant
```bash
curl -X POST http://localhost:8080/admin/tenants \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <admin-token>" \
  -d '{
    "name": "Acme Corporation"
  }'

# Response:
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "name": "Acme Corporation",
  "created_at": "2026-01-19T12:00:00Z",
  "updated_at": "2026-01-19T12:00:00Z"
}
```

#### 2. Create an Application
```bash
curl -X POST http://localhost:8080/admin/apps \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <admin-token>" \
  -d '{
    "tenant_id": "550e8400-e29b-41d4-a716-446655440000",
    "name": "Mobile App",
    "description": "iOS and Android application"
  }'

# Response:
{
  "id": "660e8400-e29b-41d4-a716-446655440000",
  "tenant_id": "550e8400-e29b-41d4-a716-446655440000",
  "name": "Mobile App",
  "description": "iOS and Android application",
  "created_at": "2026-01-19T12:05:00Z",
  "updated_at": "2026-01-19T12:05:00Z"
}
```

#### 3. Configure OAuth for Application
```bash
curl -X POST http://localhost:8080/admin/oauth-providers \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <admin-token>" \
  -d '{
    "app_id": "660e8400-e29b-41d4-a716-446655440000",
    "provider": "google",
    "client_id": "your-google-client-id.apps.googleusercontent.com",
    "client_secret": "your-google-client-secret",
    "redirect_url": "https://mobile-app.example.com/auth/google/callback",
    "is_enabled": true
  }'
```

### OAuth Configuration

**v2.0+** stores OAuth credentials in the database (per-application):
- ✅ Different OAuth credentials per application
- ✅ Runtime configuration changes (no restart needed)
- ✅ Centralized management via Admin API
- ✅ Fallback to environment variables for default app

**Migration from v1.x:**
```bash
# Migrate OAuth credentials from .env to database
go run cmd/migrate_oauth/main.go

# This reads from .env and creates database entries for:
# - Google OAuth
# - Facebook OAuth  
# - GitHub OAuth
```

### Data Isolation

**Complete isolation between applications:**
- ✅ Users are scoped to `app_id` (same email can exist in different apps)
- ✅ Social accounts linked per application
- ✅ Activity logs segmented by application
- ✅ JWT tokens include `app_id` claim (prevents cross-app token reuse)
- ✅ 2FA secrets and recovery codes isolated per app

**Database-level enforcement:**
```sql
-- Email uniqueness is per-application (not global)
CREATE UNIQUE INDEX idx_email_app_id ON users(email, app_id);

-- All user data has foreign key to applications
ALTER TABLE users ADD CONSTRAINT fk_users_app 
  FOREIGN KEY (app_id) REFERENCES applications(id) ON DELETE CASCADE;
```

### Use Cases

**SaaS Providers:**
- Serve multiple clients from single deployment
- Isolated data per client organization
- Per-client OAuth branding (different Google/Facebook apps)

**Multiple Applications:**
- Same company, different apps (mobile, web, desktop)
- Separate user bases for each platform
- Isolated analytics and audit logs

**White-Label Solutions:**
- Deploy once, serve many brands
- Customized OAuth per brand
- Complete data separation

### Upgrading from v1.x to v2.0

**📖 See:** [BREAKING_CHANGES.md](BREAKING_CHANGES.md) for complete migration guide

**Quick Summary:**
1. **Backup database** (critical!)
2. **Apply migration:** `make migrate-up`
3. **Migrate OAuth:** `go run cmd/migrate_oauth/main.go`
4. **Update API clients:** Add `X-App-ID` header to all requests
5. **Notify users:** They must re-login (JWTs invalidated)

**Data Migration:**
- All existing users → Default application
- All social accounts → Default application
- All activity logs → Default application
- Email uniqueness changes from global to per-app
- Rollback available if needed

**Breaking Changes:**
- ❌ API calls without `X-App-ID` header will fail (400 error)
- ❌ Old JWT tokens are invalid (users must re-authenticate)
- ❌ OAuth config moves from env vars to database (migration tool provided)

**📖 Detailed Guide:** [CHANGELOG.md](CHANGELOG.md#200---2026-01-19)

---

## 📊 Activity Logging System

### Overview
A professional activity logging system that balances security auditing with database performance. Uses intelligent categorization, anomaly detection, and automatic cleanup to reduce database bloat by **80-95%** while maintaining critical security data.

### Event Categories

| Severity | Events | Retention | Always Logged? |
|----------|--------|-----------|----------------|
| **CRITICAL** | LOGIN, LOGOUT, PASSWORD_CHANGE, 2FA_ENABLE/DISABLE | 1 year | ✅ Yes |
| **IMPORTANT** | REGISTER, EMAIL_VERIFY, SOCIAL_LOGIN, PROFILE_UPDATE | 6 months | ✅ Yes |
| **INFORMATIONAL** | TOKEN_REFRESH, PROFILE_ACCESS | 3 months | ⚠️ Only on anomalies |

### Anomaly Detection
Automatically logs "informational" events when:
- ✅ New IP address detected
- ✅ New device/browser (user agent) detected
- ✅ Configurable pattern analysis window (default: 30 days)

### Default Behavior
- ✅ All critical security events logged
- ✅ All important events logged
- ❌ Token refreshes NOT logged (happens every 15 minutes)
- ❌ Profile access NOT logged (happens on every view)
- ✅ BUT logs both if anomaly detected (new IP/device)
- ✅ Automatic cleanup based on retention policies

### Quick Configuration

```bash
# High-frequency events (default: disabled)
LOG_TOKEN_REFRESH=false
LOG_PROFILE_ACCESS=false

# Anomaly detection (default: enabled)
LOG_ANOMALY_DETECTION_ENABLED=true
LOG_ANOMALY_NEW_IP=true
LOG_ANOMALY_NEW_USER_AGENT=true

# Retention policies (days)
LOG_RETENTION_CRITICAL=365      # 1 year
LOG_RETENTION_IMPORTANT=180     # 6 months
LOG_RETENTION_INFORMATIONAL=90  # 3 months

# Automatic cleanup
LOG_CLEANUP_ENABLED=true
LOG_CLEANUP_INTERVAL=24h
```

**📖 Complete Guide:** [Activity Logging Documentation](docs/features/ACTIVITY_LOGGING_GUIDE.md)

---

## ⚙️ Environment Configuration

### Database
```bash
DB_HOST=postgres        # Use 'localhost' for local dev without Docker
DB_PORT=5432
DB_USER=your_db_user
DB_PASSWORD=your_db_password
DB_NAME=auth_db
```

### Redis
```bash
REDIS_ADDR=redis:6379   # Use 'localhost:6379' for local dev without Docker
REDIS_PASSWORD=         # Optional
REDIS_DB=0
```

### JWT
```bash
JWT_SECRET=your-strong-secret-key-here-change-in-production
ACCESS_TOKEN_EXPIRATION_MINUTES=15
REFRESH_TOKEN_EXPIRATION_HOURS=720  # 30 days
```

### Email
```bash
EMAIL_HOST=smtp.gmail.com
EMAIL_PORT=587
EMAIL_USERNAME=your_email@gmail.com
EMAIL_PASSWORD=your_app_password
EMAIL_FROM=noreply@yourapp.com
```

### Social Authentication Setup
```bash
# Google OAuth2
GOOGLE_CLIENT_ID=your_google_client_id
GOOGLE_CLIENT_SECRET=your_google_client_secret
GOOGLE_REDIRECT_URL=http://localhost:8080/auth/google/callback

# Facebook OAuth2
FACEBOOK_CLIENT_ID=your_facebook_app_id
FACEBOOK_CLIENT_SECRET=your_facebook_app_secret
FACEBOOK_REDIRECT_URL=http://localhost:8080/auth/facebook/callback

# GitHub OAuth2
GITHUB_CLIENT_ID=your_github_client_id
GITHUB_CLIENT_SECRET=your_github_client_secret
GITHUB_REDIRECT_URL=http://localhost:8080/auth/github/callback
```

**⚠️ v2.0+ Note:** OAuth credentials can be managed via database (Admin API) instead of environment variables. 
- Environment variables still work for default app (`00000000-0000-0000-0000-000000000001`)
- Database configuration takes precedence over env vars
- Use `go run cmd/migrate_oauth/main.go` to migrate env vars to database
- Recommended for multi-tenant deployments

### Server
```bash
PORT=8080
GIN_MODE=debug          # Use 'release' for production
```

**📖 Complete Reference:** [Environment Variables Documentation](docs/guides/ENV_VARIABLES.md)

---

## 🔧 Makefile Commands

### Development Commands
| Command | Description |
|---------|-------------|
| `make setup` | Install dependencies and development tools |
| `make dev` | Run with hot reload (Air) |
| `make run` | Run without hot reload |
| `make build` | Build binary to `bin/api.exe` |
| `make build-prod` | Build production binary (Linux, static) |
| `make clean` | Remove build artifacts and temporary files |
| `make install-air` | Install Air for hot reloading |

### Testing Commands
| Command | Description |
|---------|-------------|
| `make test` | Run all tests with verbose output |
| `make test-totp` | Run TOTP-specific test |
| `make fmt` | Format code with `go fmt` |
| `make lint` | Run linter (requires golangci-lint) |

### Docker Commands
| Command | Description |
|---------|-------------|
| `make docker-dev` | Start development environment (with hot reload) |
| `make docker-compose-up` | Start production containers |
| `make docker-compose-down` | Stop and remove all containers |
| `make docker-compose-build` | Build Docker images |
| `make docker-build` | Build single Docker image |
| `make docker-run` | Run Docker container |

### Database Migration Commands
| Command | Description |
|---------|-------------|
| `make migrate` | Interactive migration tool |
| `make migrate-status` | Check migration status (tracked in DB) |
| `make migrate-up` | Apply pending migrations |
| `make migrate-down` | Rollback last migration |
| `make migrate-list` | List all available migrations |
| `make migrate-backup` | Backup database to file |
| `make migrate-init` | Initialize migration tracking table |
| `make migrate-test` | Test migration scripts |
| `make migrate-check` | Check migration file syntax |
| `make migrate-mark-applied` | Manually mark migration as applied |

### CI/CD Commands
| Command | Description |
|---------|-------------|
| `act -j test` | Run test job locally with act |
| `act -j build` | Run build job locally with act |
| `act -j security-scan` | Run security scan locally with act |
| `act -l` | List all available GitHub Actions jobs |

**Note**: Install [act](https://github.com/nektos/act) to run GitHub Actions workflows locally for testing CI/CD pipelines before pushing.

### Security Commands
| Command | Description |
|---------|-------------|
| `make security` | Run all security checks (gosec + nancy) |
| `make security-scan` | Run gosec security scanner |
| `make vulnerability-scan` | Run nancy dependency vulnerability scanner |
| `make install-security-tools` | Install security scanning tools |

### Documentation Commands
| Command | Description |
|---------|-------------|
| `make swag-init` | Regenerate Swagger documentation |

### Help
| Command | Description |
|---------|-------------|
| `make help` | Display all available commands |

**💡 Pro Tip:** Run `make help` in your terminal to see this list with descriptions!

---

## 🗄️ Database Migrations

### Two-Tier Migration System

#### 1. GORM AutoMigrate (Automatic)
Runs on application startup:
- ✅ Creates tables from Go models
- ✅ Adds missing columns
- ✅ Creates indexes
- ✅ Safe for production
- ⚠️ Cannot handle: column renames, data transformations, complex constraints

#### 2. SQL Migrations (Manual)
For complex changes:
- ✅ Complex data transformations
- ✅ Column renames and type changes
- ✅ Custom indexes and constraints
- ✅ Performance optimizations
- ✅ Breaking changes
- ✅ Full control with rollback support

### Quick Migration Workflow

```bash
# Check current migration status
make migrate-status

# Apply all pending migrations
make migrate-up

# Rollback last migration if needed
make migrate-down

# Interactive migration tool (recommended for beginners)
make migrate
```

### For New Contributors
```bash
# 1. Start the project (GORM creates base tables automatically)
make docker-dev

# 2. Apply SQL enhancements (optional, but recommended)
make migrate-up

# 3. You're ready to develop!
make dev
```

### Creating New Migrations
```bash
# 1. Copy the template
cp migrations/TEMPLATE.md migrations/YYYYMMDD_HHMMSS_your_migration.md

# 2. Create forward migration SQL
# migrations/YYYYMMDD_HHMMSS_your_migration.sql

# 3. Create rollback SQL
# migrations/YYYYMMDD_HHMMSS_your_migration_rollback.sql

# 4. Test and apply
make migrate-test
make migrate-up
```

**📖 Complete Guide:** [Migration System Documentation](docs/migrations/README.md)

---

## 🧪 Testing

### Run Tests
```bash
# All tests with verbose output
make test

# Specific package
go test -v ./internal/auth/...

# With coverage report
go test -cover ./...

# 2FA TOTP test (requires TEST_TOTP_SECRET env var)
make test-totp
```

### Manual API Testing
```bash
# Using the test script
./test_api.sh

# Or use interactive Swagger UI
# Navigate to: http://localhost:8080/swagger/index.html
```

### Test Coverage
The project includes:
- ✅ Unit tests for core logic
- ✅ Integration tests for API endpoints
- ✅ 2FA/TOTP verification tests
- ✅ Authentication flow tests
- ✅ Database operation tests

---

## 🏗️ Project Structure

```
project-root/
├── cmd/
│   ├── api/                    # Application entry point
│   │   └── main.go
│   └── migrate_oauth/          # OAuth migration tool (v2.0+)
│       └── main.go
├── internal/                   # Private application code
│   ├── admin/                 # Admin API (multi-tenancy management)
│   ├── auth/                  # Authentication handlers
│   ├── user/                  # User management
│   ├── social/                # Social OAuth2 providers
│   ├── twofa/                 # Two-factor authentication
│   ├── log/                   # Activity logging system
│   ├── email/                 # Email verification & reset
│   ├── middleware/            # JWT auth, AppID, CORS middleware
│   ├── database/              # Database connection & migrations
│   ├── redis/                 # Redis connection & operations
│   ├── config/                # Configuration management
│   └── util/                  # Utility functions
├── pkg/                        # Public packages
│   ├── models/                # Database models (GORM)
│   │   ├── tenant.go          # v2.0+
│   │   ├── application.go     # v2.0+
│   │   ├── oauth_provider_config.go  # v2.0+
│   │   └── ...
│   ├── dto/                   # Data transfer objects
│   │   ├── admin.go           # v2.0+ Admin DTOs
│   │   └── ...
│   ├── errors/                # Custom error types
│   └── jwt/                   # JWT token utilities
├── docs/                       # Documentation
│   ├── features/              # Feature-specific docs
│   ├── guides/                # Setup and configuration guides
│   ├── migrations/            # Migration system docs
│   ├── implementation/        # Implementation details
│   ├── implementation_phases/ # Original project phases
│   ├── API.md                 # API reference
│   ├── ARCHITECTURE.md        # System architecture
│   └── README.md              # Documentation index
├── migrations/                 # SQL migration files
│   ├── README.md              # Developer migration guide
│   ├── TEMPLATE.md            # Migration template
│   ├── 20260105_add_multi_tenancy.sql  # v2.0+ Multi-tenancy
│   └── *.sql                  # Migration scripts
├── scripts/                    # Helper scripts
│   ├── migrate.sh             # Migration runner (Unix)
│   ├── migrate.bat            # Migration runner (Windows)
│   ├── backup_db.sh           # Database backup (Unix)
│   ├── backup_db.bat          # Database backup (Windows)
│   ├── apply_pending_migrations.sh
│   ├── rollback_last_migration.sh
│   └── cleanup_activity_logs.sh
├── .github/                    # GitHub configuration
│   ├── copilot-instructions.md # AI coding assistant instructions
│   ├── ISSUE_TEMPLATE/        # Issue templates
│   └── workflows/             # CI/CD workflows
├── Dockerfile                  # Production Docker image
├── Dockerfile.dev              # Development Docker image
├── docker-compose.yml          # Production compose config
├── docker-compose.dev.yml      # Development compose config
├── Makefile                    # Build and development commands
├── .air.toml                   # Hot reload configuration
├── .env.example                # Environment variables template
├── go.mod                      # Go module dependencies
├── go.sum                      # Dependency checksums
├── AGENTS.md                   # AI agent coding guidelines
├── CONTRIBUTING.md             # Contribution guidelines
├── CODE_OF_CONDUCT.md          # Code of conduct
├── SECURITY.md                 # Security policy
├── CHANGELOG.md                # Version history
├── BREAKING_CHANGES.md         # Breaking changes tracker (v2.0+)
├── docs/migrations/MIGRATIONS.md  # Migration system overview
├── LICENSE                     # MIT License
└── README.md                   # This file
```

---

## 🛠️ Tech Stack

| Category | Technology |
|----------|-----------|
| **Language** | Go 1.23+ |
| **Web Framework** | [Gin](https://github.com/gin-gonic/gin) |
| **Database** | PostgreSQL 13+ with [GORM](https://gorm.io/) ORM |
| **Cache & Sessions** | Redis 6+ with [go-redis](https://github.com/redis/go-redis) |
| **Authentication** | JWT (golang-jwt/jwt), OAuth2 |
| **2FA** | TOTP ([pquerna/otp](https://github.com/pquerna/otp)), QR codes |
| **Validation** | [go-playground/validator](https://github.com/go-playground/validator) |
| **Email** | [gopkg.in/mail.v2](https://gopkg.in/mail.v2) (SMTP) |
| **Configuration** | [Viper](https://github.com/spf13/viper), [godotenv](https://github.com/joho/godotenv) |
| **API Documentation** | [Swagger/Swaggo](https://github.com/swaggo/swag) |
| **Development** | [Air](https://github.com/air-verse/air) (hot reload) |
| **Containerization** | Docker, Docker Compose |
| **Security Tools** | gosec, nancy |

---

## 🤝 Contributing

We welcome contributions! Here's how to get started:

### 1. Read the Guidelines
   - [CONTRIBUTING.md](CONTRIBUTING.md) - Contribution process and standards
   - [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md) - Community guidelines

### 2. Fork & Clone
   ```bash
   git clone https://github.com/yourusername/auth-api.git
   cd auth-api
   ```

### 3. Set Up Development Environment
   ```bash
   # Install dependencies and tools
   make setup
   
   # Copy and configure environment
   cp .env.example .env
   # Edit .env with your settings
   
   # Start development environment
   make docker-dev
   ```

### 4. Create a Branch
   ```bash
   git checkout -b feature/amazing-feature
   # or
   git checkout -b fix/bug-description
   ```

### 5. Make Changes & Test
   ```bash
   # Format code
   make fmt
   
   # Run linter
   make lint
   
   # Run tests
   make test
   
   # Security checks
   make security
   ```

### 6. Commit Your Changes
   ```bash
   git commit -m "feat(auth): add amazing feature"
   ```
   
   Follow [Conventional Commits](https://www.conventionalcommits.org/):
   - `feat(scope): description` - New feature
   - `fix(scope): description` - Bug fix
   - `docs(scope): description` - Documentation
   - `refactor(scope): description` - Code refactoring
   - `test(scope): description` - Tests
   - `chore(scope): description` - Maintenance

### 7. Push & Create Pull Request
   ```bash
   git push origin feature/amazing-feature
   ```
   Then create a Pull Request on GitHub

### Development Workflow
```bash
# Daily development
make dev              # Start with hot reload
make test             # Run tests
make fmt && make lint # Format and check code
make security         # Security checks before commit
```

---

## 🛡️ Security

### Reporting Vulnerabilities
**Please DO NOT create public issues for security vulnerabilities.**

Read [SECURITY.md](SECURITY.md) for instructions on how to report security vulnerabilities privately.

### Security Features
- ✅ **JWT Authentication** with access & refresh tokens
- ✅ **Token Blacklisting** on logout for immediate invalidation
- ✅ **Password Hashing** using bcrypt
- ✅ **Two-Factor Authentication** with TOTP and recovery codes
- ✅ **Email Verification** for account security
- ✅ **Rate Limiting** (configurable)
- ✅ **SQL Injection Protection** (GORM parameterized queries)
- ✅ **XSS Protection** (input validation and sanitization)
- ✅ **CORS Configuration** (customizable)
- ✅ **Activity Logging** and comprehensive audit trails
- ✅ **Security Headers** (recommended middleware)

### Security Tools & Scanning
```bash
# Run all security checks
make security

# Individual scans
make security-scan         # gosec - Go security checker
make vulnerability-scan    # nancy - dependency vulnerability scanner

# Install security tools
make install-security-tools
```

---

## 📄 License

This project is licensed under the **MIT License** - see the [LICENSE](LICENSE) file for details.

---

## 📞 Support & Resources

### Documentation
- 📖 **[Complete Documentation](docs/README.md)** - All documentation organized by topic
- 🏗️ **[Architecture Guide](docs/ARCHITECTURE.md)** - System design and patterns
- 📡 **[API Reference](docs/API.md)** - Detailed endpoint documentation

### Community
- 🐛 **[GitHub Issues](https://github.com/yourusername/auth-api/issues)** - Bug reports and feature requests
- 💬 **[GitHub Discussions](https://github.com/yourusername/auth-api/discussions)** - Questions and discussions
- 🤝 **[Contributing Guide](CONTRIBUTING.md)** - How to contribute

### Getting Help
1. Check the [documentation](docs/README.md)
2. Search [existing issues](https://github.com/yourusername/auth-api/issues)
3. Create a new issue with details
4. Join discussions for general questions

---

## 🙏 Acknowledgments

Built with modern Go practices and industry-standard security patterns.

### Special Thanks To:
- [Gin Web Framework](https://github.com/gin-gonic/gin) - Fast HTTP web framework
- [GORM](https://gorm.io/) - Powerful ORM library
- [Swaggo](https://github.com/swaggo/swag) - Swagger documentation generator
- [Air](https://github.com/air-verse/air) - Live reload for Go apps
- All open-source contributors and maintainers

---

<div align="center">

**Made with ❤️ using Go**

[⬆ Back to Top](#-authentication-api)

</div>
