#!/bin/bash

# Get the directory where the script is located
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
BACKUP_DIR="$PROJECT_ROOT/backups"

# Create backups directory if it doesn't exist
mkdir -p "$BACKUP_DIR"

# Load .env file
if [ -f "$PROJECT_ROOT/.env" ]; then
    # Export variables from .env, ignoring comments
    export $(grep -v '^#' "$PROJECT_ROOT/.env" | xargs)
fi

# Default values if not set in .env
DB_USER=${DB_USER:-postgres}
DB_PASSWORD=${DB_PASSWORD:-root}
DB_NAME=${DB_NAME:-auth_db}
CONTAINER_NAME="auth_db"

# Timestamp for the backup file
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
BACKUP_FILE="$BACKUP_DIR/${DB_NAME}_backup_$TIMESTAMP.sql"

echo "Creating backup of $DB_NAME..."
echo "Target file: $BACKUP_FILE"

# Check if container is running
if ! docker ps | grep -q "$CONTAINER_NAME"; then
    echo "Error: $CONTAINER_NAME container is not running."
    exit 1
fi

# Execute pg_dump inside the container
docker exec -e PGPASSWORD="$DB_PASSWORD" -t "$CONTAINER_NAME" pg_dump -U "$DB_USER" "$DB_NAME" > "$BACKUP_FILE"

if [ $? -eq 0 ]; then
    echo "Backup created successfully!"
    if [[ "$OSTYPE" == "msys" || "$OSTYPE" == "cygwin" ]]; then
        # Windows/Git Bash
        du -h "$BACKUP_FILE" | cut -f1
    else
        # Linux/Mac
        du -h "$BACKUP_FILE" | cut -f1
    fi
else
    echo "Backup failed!"
    rm -f "$BACKUP_FILE"
    exit 1
fi
