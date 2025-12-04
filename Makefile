# Auth API Makefile

.PHONY: build run dev test clean air

# Build the application
build:
	go build -o bin/api.exe ./cmd/api

# Run the application without hot reload
run:
	go run ./cmd/api

# Run with hot reload using Air
dev:
	air

# Run tests
test:
	go test -v ./...

# Run the TOTP test (requires TEST_TOTP_SECRET environment variable)
test-totp:
	@if [ -z "$$TEST_TOTP_SECRET" ]; then \
		echo "Error: TEST_TOTP_SECRET environment variable is required"; \
		echo "Set it with: export TEST_TOTP_SECRET=your_secret_here"; \
		echo "Or run: ./run_test_secret.sh"; \
		exit 1; \
	fi
	go run test_specific_secret.go

# Clean build artifacts and temporary files
clean:
	rm -rf bin/
	rm -rf tmp/
	go clean

# Install Air for hot reloading
install-air:
	go install github.com/air-verse/air@latest

# Setup development environment
setup: install-air
	go mod tidy
	go mod download

# Check code formatting
fmt:
	go fmt ./...

# Run linter (requires golangci-lint)
lint:
	golangci-lint run

# Install security scanning tools
install-security-tools:
	go install github.com/securego/gosec/v2/cmd/gosec@latest
	go install github.com/sonatype-nexus-community/nancy@latest

# Run security scan with gosec
security-scan:
	gosec -conf .gosec.json ./...

# Run vulnerability scan with nancy
vulnerability-scan:
	go list -json -deps ./... | nancy sleuth

# Run all security checks
security: security-scan vulnerability-scan

# Build for production
build-prod:
	CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o bin/api ./cmd/api

# Docker commands

# Run development environment with Docker
docker-dev:
	./dev.sh

# Run production environment with Docker
docker-compose-build:
	docker-compose build

# Stop and remove containers, networks, images, and volumes	
docker-compose-down:
	docker-compose down

# Start Docker containers in detached mode with build
docker-compose-up:
	docker-compose up -d --build

# Docker build
docker-build:
	docker build -t auth-api .

# Docker run
docker-run:
	docker run -p 8080:8080 --env-file .env auth-api

# Generate documentation using Swagger
swag-init:
	swag init -g cmd/api/main.go -o docs

# Database Migration commands

# Show migration status
migrate-status:
	@echo "Running migration status check..."
	@docker exec -it auth_db psql -U postgres -d auth_db -c "\dt" || echo "Database not running. Start with: make docker-dev"

# Apply all pending migrations (Docker-aware)
migrate-up:
	@echo "Applying migrations..."
	@if [ ! -f "migrations/20240103_add_activity_log_smart_fields.sql" ]; then \
		echo "Error: Migration file not found!"; \
		exit 1; \
	fi
	@echo "Applying: 20240103_add_activity_log_smart_fields.sql"
	@docker exec -i auth_db psql -U postgres -d auth_db < migrations/20240103_add_activity_log_smart_fields.sql
	@echo "✅ Migrations applied successfully!"

# Rollback last migration (Docker-aware)
migrate-down:
	@echo "Rolling back migration..."
	@if [ ! -f "migrations/20240103_add_activity_log_smart_fields_rollback.sql" ]; then \
		echo "Error: Rollback file not found!"; \
		exit 1; \
	fi
	@echo "Rolling back: 20240103_add_activity_log_smart_fields_rollback.sql"
	@docker exec -i auth_db psql -U postgres -d auth_db < migrations/20240103_add_activity_log_smart_fields_rollback.sql
	@echo "✅ Rollback completed!"

# List available migrations
migrate-list:
	@echo "Available migrations:"
	@ls -1 migrations/*.sql 2>/dev/null || echo "No migrations found"

# Create database backup (Docker-aware)
migrate-backup:
	@echo "Creating database backup..."
	@mkdir -p backups
	@docker exec auth_db pg_dump -U postgres auth_db > backups/backup_$(shell date +%Y%m%d_%H%M%S).sql
	@echo "✅ Backup created in backups/ directory"

# Test database connection (Docker-aware)
migrate-test:
	@echo "Testing database connection..."
	@docker exec auth_db psql -U postgres -d auth_db -c "SELECT version();" && echo "✅ Connection successful!" || echo "❌ Connection failed. Start with: make docker-dev"

# Check database tables and schema (Docker-aware)
migrate-check:
	@echo "Checking database schema..."
	@echo "\n📋 Tables:"
	@docker exec auth_db psql -U postgres -d auth_db -c "\dt"
	@echo "\n📊 Activity Logs Structure:"
	@docker exec auth_db psql -U postgres -d auth_db -c "\d activity_logs"

# Interactive migration tool (if you have psql locally)
migrate:
	@./scripts/migrate.sh

# Initialize migration tracking (first time only)
migrate-init:
	@echo "Initializing migration tracking..."
	@docker exec -i auth_db psql -U postgres -d auth_db < migrations/00_create_migrations_table.sql
	@echo "✅ Migration tracking initialized!"

# Check which migrations are tracked in database
migrate-status-tracked:
	@echo "📋 Migrations recorded in database:"
	@docker exec auth_db psql -U postgres -d auth_db -c "SELECT version, name, applied_at, execution_time_ms || 'ms' as duration FROM schema_migrations ORDER BY applied_at;" 2>/dev/null || echo "⚠️  Tracking not initialized. Run: make migrate-init"

# Mark migration as applied manually
migrate-mark-applied:
	@if [ -z "$(VERSION)" ] || [ -z "$(NAME)" ]; then \
		echo "Usage: make migrate-mark-applied VERSION=20240103_000000 NAME=\"description\""; \
		exit 1; \
	fi
	@docker exec auth_db psql -U postgres -d auth_db -c \
		"INSERT INTO schema_migrations (version, name, success) VALUES ('$(VERSION)', '$(NAME)', true) ON CONFLICT (version) DO NOTHING;"
	@echo "✅ Migration $(VERSION) marked as applied"

# Show help
help:
	@echo "Available commands:"
	@echo "  build                - Build the application"
	@echo "  run                  - Run the application"
	@echo "  dev                  - Run with hot reload (Air)"
	@echo "  test                 - Run tests"
	@echo "  test-totp            - Run TOTP test (requires TEST_TOTP_SECRET env var)"
	@echo "  clean                - Clean build artifacts"
	@echo "  install-air          - Install Air for hot reloading"
	@echo "  setup                - Setup development environment"
	@echo "  fmt                  - Format code"
	@echo "  lint                 - Run linter"
	@echo "  install-security-tools - Install gosec and nancy security scanners"
	@echo "  security-scan        - Run gosec security scanner"
	@echo "  vulnerability-scan   - Run nancy vulnerability scanner"
	@echo "  security             - Run all security checks"
	@echo "  build-prod           - Build for production"
	@echo "  docker-dev           - Run development environment with Docker"
	@echo "  docker-compose-build - Build Docker images using docker-compose"
	@echo "  docker-compose-down  - Stop and remove Docker containers, networks, images, volumes"
	@echo "  docker-compose-up    - Start Docker containers in detached mode with build"
	@echo "  docker-build         - Build Docker image (auth-api)"
	@echo "  docker-run           - Run Docker container with environment from .env"
	@echo "  swag-init            - Generate Swagger documentation (docs/)"
	@echo ""
	@echo "Database Migration commands (Docker-compatible):"
	@echo "  migrate-status       - Show database tables"
	@echo "  migrate-up           - Apply pending migrations (Docker)"
	@echo "  migrate-down         - Rollback last migration (Docker)"
	@echo "  migrate-check        - Check database schema details"
	@echo "  migrate-backup       - Create database backup (Docker)"
	@echo "  migrate-test         - Test database connection (Docker)"
	@echo "  migrate-list         - List available migration files"
	@echo "  migrate              - Interactive tool (requires local psql)"
	@echo ""
	@echo "Migration Tracking (Advanced):"
	@echo "  migrate-init         - Initialize migration tracking table"
	@echo "  migrate-status-tracked - Show tracked migrations in database"
	@echo "  migrate-mark-applied - Mark migration as applied (VERSION=... NAME=...)"