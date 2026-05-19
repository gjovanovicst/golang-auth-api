#!/bin/bash

echo "Starting Authentication API in Development Mode with Hot Reload..."
echo ""
echo "Services will be available at:"
echo "- Auth API: http://localhost:8080"
echo "- Redis Commander: http://localhost:8081"
echo "- PostgreSQL: localhost:5433"
echo ""

# Prefer podman-compose (bypasses Docker Desktop), then podman compose, then docker compose
if command -v podman-compose &> /dev/null; then
  COMPOSE_CMD="podman-compose"
elif command -v podman &> /dev/null; then
  COMPOSE_CMD="podman compose"
elif command -v docker &> /dev/null && docker compose version &> /dev/null; then
  COMPOSE_CMD="docker compose"
else
  COMPOSE_CMD="docker-compose"
fi

echo "Using compose command: $COMPOSE_CMD"

# Ensure the shared external network exists
podman network create shared-api-network 2>/dev/null || true

# Tear down any previous run first to avoid stale container-name / pod conflicts
$COMPOSE_CMD -f docker-compose.dev.yml down 2>/dev/null || true

# Build the dev image explicitly — podman-compose ignores the dockerfile: key
# so we must build it ourselves and reference it by image name in the compose file.
echo "Building dev image from Dockerfile.dev..."
podman build -t localhost/auth-dev_auth-api:latest -f Dockerfile.dev .

$COMPOSE_CMD -f docker-compose.dev.yml up
