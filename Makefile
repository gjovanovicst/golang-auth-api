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