# Quickstart

This page takes you from an empty directory to a working token in about five minutes.

## Prerequisites

- Docker and Docker Compose (recommended), **or** Go 1.23+, PostgreSQL 15+ and Redis 7+
- `curl` (or any HTTP client)

## 1. Configure the environment

```bash
git clone https://github.com/gjovanovicst/golang-auth-api.git
cd golang-auth-api
cp .env.example .env
```

Edit `.env` and set at least:

```bash
DB_PASSWORD=change-me
JWT_SECRET=a-random-string-of-at-least-32-characters
ADMIN_API_KEY=a-random-admin-key
```

## 2. Start PostgreSQL and Redis

```bash
docker compose up -d postgres redis
```

The compose file publishes PostgreSQL on **5433** and Redis on **6379**, which is what
`docker-compose.yml` and `.env.example` expect. If you use your own database, point
`DB_HOST`/`DB_PORT` at it instead.

## 3. Start the API

```bash
make dev        # hot reload with Air
# or
make run        # plain `go run ./cmd/api`
```

On boot the service connects to PostgreSQL and Redis, applies pending GORM
auto-migrations, seeds nothing else, and listens on `:8080`. The default application
`00000000-0000-0000-0000-000000000001` is created by the multi-tenancy migration.

> **Docker only:** `docker compose up -d --build` builds and runs the API in a
> container as well, with `DB_HOST=postgres` and `REDIS_ADDR=redis:6379`.

## 4. Create the admin account

```bash
make setup-admin
# or non-interactive:
go run ./cmd/setup -username admin -password 'a-strong-password'
```

Then open the admin GUI at <http://localhost:8080/gui/login>.

## 5. Register and log in

Every API call needs the application header:

```bash
export APP=00000000-0000-0000-0000-000000000001

curl -sX POST http://localhost:8080/register \
  -H "X-App-ID: $APP" -H "Content-Type: application/json" \
  -d '{"email":"user@example.com","password":"Str0ng!Passw0rd"}'

curl -sX POST http://localhost:8080/login \
  -H "X-App-ID: $APP" -H "Content-Type: application/json" \
  -d '{"email":"user@example.com","password":"Str0ng!Passw0rd"}'
```

The login response contains `access_token` and `refresh_token`. If the user has 2FA
enabled, it instead returns `requires_2fa: true` plus a short-lived `temp_token` that
you submit to `/2fa/login-verify`.

## 6. Look around

| URL | What |
|---|---|
| <http://localhost:8080/health> | Health check (needs `X-App-ID`) |
| <http://localhost:8080/swagger/index.html> | Interactive API docs |
| <http://localhost:8080/gui/login> | Admin GUI |
| <http://localhost:8080/app-config/:app_id> | Public login configuration for a frontend |

## 7. Optional: seed development users

```powershell
./scripts/seed-dev-users.ps1
```

Creates `dev@example.com` (no 2FA) and `dev-2fa@example.com` (TOTP + verified phone)
with known passwords, so you can exercise the login and 2FA flows immediately.

## Troubleshooting

| Symptom | Cause |
|---|---|
| `400 {"error":"X-App-ID header is required"}` | The request is missing the `X-App-ID` header (Swagger, `/admin/*`, `/gui/*` and OIDC routes excepted) |
| `500 Failed to connect to database` | Wrong `DB_*` values, or PostgreSQL is not running / not migrated |
| `500 Could not connect to Redis` | `REDIS_ADDR` is wrong or Redis is not running |
| Emails are not delivered | No SMTP configuration — the API prints emails to the log in development mode (see [Email](21-email.md)) |
