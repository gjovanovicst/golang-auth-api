#!/bin/bash
set -e

DB_CONTAINER="auth_db"
DB_USER="postgres"
DB_NAME="auth_db"

# Detect container runtime with connectivity check.
# Honour an explicit override passed from the Makefile or the caller's environment.
if [ -z "${CONTAINER_CMD}" ]; then
  if command -v podman &> /dev/null && podman ps > /dev/null 2>&1; then
    CONTAINER_CMD="podman"
  elif command -v docker &> /dev/null; then
    CONTAINER_CMD="docker"
  else
    echo "❌ No container runtime found. Install Docker or start Podman (podman machine start)."
    exit 1
  fi
fi

echo "Using container runtime: ${CONTAINER_CMD}"
echo "Checking for migrations to rollback..."

# 1. Get the most recently applied migration
LAST_VERSION=$($CONTAINER_CMD exec -i $DB_CONTAINER psql -U $DB_USER -d $DB_NAME -t -c "SELECT version FROM schema_migrations ORDER BY applied_at DESC LIMIT 1" 2>/dev/null | xargs || echo "")

if [ -z "$LAST_VERSION" ]; then
    echo "No applied migrations found to rollback."
    exit 0
fi

echo "Last applied migration: $LAST_VERSION"

# 2. Look for the corresponding rollback file
# Supported patterns:
# - migrations/{version}_rollback.sql
# - migrations/rollback_{version}.sql
ROLLBACK_FILE="migrations/${LAST_VERSION}_rollback.sql"

if [ ! -f "$ROLLBACK_FILE" ]; then
    # Try alternative naming if needed, or just fail
    echo "❌ Rollback file not found: $ROLLBACK_FILE"
    echo "Cannot automatically rollback this migration."
    exit 1
fi

echo "Rolling back using: $ROLLBACK_FILE"

# 3. Execute Rollback
if $CONTAINER_CMD exec -i $DB_CONTAINER psql -U $DB_USER -d $DB_NAME -v ON_ERROR_STOP=1 < "$ROLLBACK_FILE"; then
    # 4. Remove record from schema_migrations
    $CONTAINER_CMD exec -i $DB_CONTAINER psql -U $DB_USER -d $DB_NAME -c "DELETE FROM schema_migrations WHERE version = '$LAST_VERSION';" > /dev/null
    echo "✅ Rollback successful for $LAST_VERSION"
else
    echo "❌ Rollback failed"
    exit 1
fi
