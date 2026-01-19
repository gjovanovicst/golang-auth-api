# AI Coding Instructions for Auth API

## 🔍 Project Context & Architecture

- **Type**: Modular Monolith Go REST API using **Gin** framework.
- **Data Layer**: **PostgreSQL** with **GORM**, **Redis** for caching/sessions.
- **Structure**: Clean Architecture-ish.
  - `cmd/api/main.go`: Entry point, dependency injection wire-up.
  - `internal/<domain>/`: Encapsulated features (e.g., `user`, `social`, `log`).
  - `internal/<domain>/handler.go`: HTTP transport handling.
  - `internal/<domain>/service.go`: Business rules.
  - `internal/<domain>/repository.go`: Database interactions.
  - `pkg/`: Shared code (DTOs, simplified models, utils).

## 🛠️ Build & Test Workflows

- **Development**: Use `make dev` to run with hot-reload (Air).
- **Testing**: Run `make test` for unit tests.
- **Build**: `make build` generates generic binaries.
- **Database**:
  - Migrations run automatically on startup (`database.MigrateDatabase()`).
  - Use `scripts/backup_db.sh` for backups.

## 📝 Conventions & Patterns

- **Dependency Injection**: Manual DI in `main.go`. Initialize Repo -> Service -> Handler.
- **Configuration**: **Viper** + `.env`. Check `cmd/api/main.go` for default key setups.
- **Validation**: Use `go-playground/validator` struct tags in DTOs.
- **Documentation**: Update Swagger comments (`// @Summary ...`) when changing handlers. Run `swag init` (often implied or manual) to regenerate docs in `docs/`.
- **Security**:
  - Use `internal/middleware` for JWT validation & RBAC.
  - Don't expose raw GORM errors to API clients; wrap them.
- **Logging**: Use `internal/log` service for audit trails (critical/important/info).

## ⚠️ Important Implementation Details

- **Social Auth**: Handlers in `internal/social` manage OAuth flows (Google, Github, FB).
- **2FA**: `internal/twofa` handles TOTP logic.
- **Redis**: Essential for token blacklisting and session data; ensure Redis is available.
- **Database Modals**: GORM models often live in domain packages.

## 🚀 Examples

- **Adding a new endpoint**:
  1. Define DTO in `pkg/dto`.
  2. add function to `Repository` interface & impl in `internal/<domain>/repository.go`.
  3. Add logic to `Service` in `internal/<domain>/service.go`.
  4. Register route in `Handler` and `main.go`.
- **Log Activity**:
  ```go
  logQueryService.CreateLog(ctx, "USER_LOGIN", "User logged in", userID, "INFO")
  ```
