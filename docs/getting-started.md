# Getting Started

Step-by-step guide to set up and run the Authentication API.

---

## Prerequisites

- **Docker & Docker Compose** (recommended)
- Or install locally: Go 1.23+, PostgreSQL 13+, Redis 6+

---

## Installation

```bash
# 1. Clone the repository
git clone <repository-url>
cd <project-directory>

# 2. Copy environment configuration
cp .env.example .env
# Edit .env with your database, Redis, and JWT settings

# 3. Create shared Docker network (first time only)
./setup-network.sh create

# 4. Start with Docker (recommended)
make docker-dev
# Windows: dev.bat | Linux/Mac: ./dev.sh

# 5. Apply database migrations
make migrate-up

# 6. (Optional) Migrate OAuth credentials to database
go run cmd/migrate_oauth/main.go
```

Your API is now running at `http://localhost:8080`.

---

## What Gets Created

After starting for the first time:

- PostgreSQL and Redis containers running via Docker
- Database tables created (multi-tenant architecture)
- Default tenant and application created (`00000000-0000-0000-0000-000000000001`)
- Hot reload enabled for development
- Swagger docs available at [http://localhost:8080/swagger/index.html](http://localhost:8080/swagger/index.html)

---

## Required Header

All API requests (except Swagger, Admin, and OAuth callbacks) require the `X-App-ID` header:

```bash
curl -X POST http://localhost:8080/auth/register \
  -H "X-App-ID: 00000000-0000-0000-0000-000000000001" \
  -H "Content-Type: application/json" \
  -d '{"email":"user@example.com","password":"Pass123!@#"}'
```

The default Application ID `00000000-0000-0000-0000-000000000001` is created automatically on first run.

---

## Next Steps

| Topic | Link |
|-------|------|
| Configure environment variables | [Configuration](configuration.md) |
| Set up social OAuth providers | [Configuration - OAuth](configuration.md#social-authentication) |
| Multi-tenancy guide | [Multi-Tenancy](multi-tenancy.md) |
| Admin GUI setup | [Admin GUI](admin-gui.md) |
| Activity logging | [Activity Logging](activity-logging.md) |
| Database migrations | [Database Migrations](database-migrations.md) |
| API endpoints reference | [API Endpoints](api-endpoints.md) |

---

## Development Workflow

```bash
make dev              # Start with hot reload
make test             # Run tests
make fmt && make lint # Format and lint code
make security         # Security checks before committing
```

For the full list of available commands, see the [Makefile Reference](makefile-reference.md).
