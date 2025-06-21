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

# Build for production
build-prod:
	CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o bin/api ./cmd/api

# Docker build
docker-build:
	docker build -t auth-api .

# Docker run
docker-run:
	docker run -p 8080:8080 --env-file .env auth-api

# Show help
help:
	@echo "Available commands:"
	@echo "  build      - Build the application"
	@echo "  run        - Run the application"
	@echo "  dev        - Run with hot reload (Air)"
	@echo "  test       - Run tests"
	@echo "  clean      - Clean build artifacts"
	@echo "  setup      - Setup development environment"
	@echo "  fmt        - Format code"
	@echo "  lint       - Run linter"
	@echo "  build-prod - Build for production"
	@echo "  docker-*   - Docker commands"