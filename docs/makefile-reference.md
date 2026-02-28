# Makefile Reference

Run `make help` to see all available commands with descriptions.

---

## Development

| Command | Description |
|---------|-------------|
| `make setup` | Install dependencies and development tools |
| `make dev` | Run with hot reload (Air) |
| `make run` | Run without hot reload |
| `make build` | Build binary to `bin/api.exe` |
| `make build-prod` | Build production binary (Linux, static) |
| `make clean` | Remove build artifacts and temporary files |
| `make install-air` | Install Air for hot reloading |

---

## Testing

| Command | Description |
|---------|-------------|
| `make test` | Run all tests with verbose output |
| `make test-totp` | Run TOTP-specific test |
| `make fmt` | Format code with `go fmt` |
| `make lint` | Run linter (requires golangci-lint) |

---

## Docker

| Command | Description |
|---------|-------------|
| `make docker-dev` | Start development environment (with hot reload) |
| `make docker-compose-up` | Start production containers |
| `make docker-compose-down` | Stop and remove all containers |
| `make docker-compose-build` | Build Docker images |
| `make docker-build` | Build single Docker image |
| `make docker-run` | Run Docker container |

---

## Database Migrations

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

---

## Security

| Command | Description |
|---------|-------------|
| `make security` | Run all security checks (gosec + nancy) |
| `make security-scan` | Run gosec security scanner |
| `make vulnerability-scan` | Run nancy dependency vulnerability scanner |
| `make install-security-tools` | Install security scanning tools |

---

## CI/CD (Local Testing with Act)

| Command | Description |
|---------|-------------|
| `act -j test` | Run test job locally |
| `act -j build` | Run build job locally |
| `act -j security-scan` | Run security scan locally |
| `act -l` | List all available GitHub Actions jobs |

Install [act](https://github.com/nektos/act) to run GitHub Actions workflows locally.

---

## Documentation

| Command | Description |
|---------|-------------|
| `make swag-init` | Regenerate Swagger documentation |
