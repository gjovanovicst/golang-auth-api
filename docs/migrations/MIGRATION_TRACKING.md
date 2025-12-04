# Migration Tracking System

## Overview

The system tracks which migrations have been applied using a `schema_migrations` table in the database.

---

## Quick Start

### Initialize Tracking (First Time Only)

```bash
# Initialize migration tracking table
make migrate-init

# OR manually
docker exec -i auth_db psql -U postgres -d auth_db < migrations/00_create_migrations_table.sql
```

### Check What's Applied

```bash
# Check migration status
make migrate-status-tracked

# Shows which migrations are in the database
```

### Apply Migrations with Tracking

```bash
# Apply and record in database
make migrate-up-tracked
```

---

## The schema_migrations Table

### Structure

```sql
CREATE TABLE schema_migrations (
    id SERIAL PRIMARY KEY,
    version VARCHAR(255) UNIQUE,      -- e.g., "20240103_120000"
    name VARCHAR(255),                -- e.g., "add_activity_log_smart_fields"
    applied_at TIMESTAMP,             -- When it was applied
    execution_time_ms INTEGER,        -- How long it took
    success BOOLEAN,                  -- Did it succeed?
    error_message TEXT,               -- Error if failed
    checksum VARCHAR(64)              -- SHA256 of migration file
);
```

### Example Data

| version | name | applied_at | execution_time_ms | success |
|---------|------|------------|-------------------|---------|
| 00000000_000000 | create_migrations_table | 2024-01-03 10:00:00 | 0 | true |
| 20240103_000000 | add_activity_log_smart_fields | 2024-01-03 10:01:00 | 1234 | true |

---

## How It Works

### When You Run Migrations

1. Script checks `schema_migrations` table
2. Compares with files in `migrations/` folder
3. Identifies pending migrations (not in table)
4. Applies pending migrations
5. Records each migration in the table

### Checking Status

```bash
# Via SQL
docker exec auth_db psql -U postgres -d auth_db -c \
  "SELECT version, name, applied_at FROM schema_migrations ORDER BY applied_at;"

# Via Go code
var migrations []models.SchemaMigration
db.Order("applied_at DESC").Find(&migrations)
```

---

## Migration Commands

### Using Makefile (Recommended)

```bash
# Initialize tracking (first time)
make migrate-init

# Check status
make migrate-status-tracked

# Apply pending migrations
make migrate-up-tracked

# Mark migration as applied (manual)
make migrate-mark-applied VERSION=20240103_000000 NAME="description"
```

### Manual Docker Commands

```bash
# Initialize
docker exec -i auth_db psql -U postgres -d auth_db < migrations/00_create_migrations_table.sql

# Check applied migrations
docker exec auth_db psql -U postgres -d auth_db -c \
  "SELECT * FROM schema_migrations ORDER BY applied_at;"

# Check pending migrations
docker exec auth_db psql -U postgres -d auth_db -c \
  "SELECT version FROM schema_migrations;" | grep -v "version\|---\|row"

# Apply specific migration and record it
docker exec -i auth_db psql -U postgres -d auth_db < migrations/20240103_add_activity_log_smart_fields.sql
docker exec auth_db psql -U postgres -d auth_db -c \
  "INSERT INTO schema_migrations (version, name) VALUES ('20240103_000000', 'add_activity_log_smart_fields');"
```

---

## Benefits

### Know What's Applied
- Database knows its own state
- No confusion about what's been run
- Team coordination easier

### Prevent Re-Running
- Script automatically skips applied migrations
- Safe to run multiple times
- No accidental duplicates

### Audit Trail
- When was each migration applied?
- How long did it take?
- Did it succeed or fail?
- Complete history

### CI/CD Friendly
```bash
# Check for pending migrations
if docker exec auth_db psql -U postgres -d auth_db -c \
  "SELECT COUNT(*) FROM schema_migrations WHERE success = false;" | grep -q "0"; then
    echo "All migrations successful"
else
    echo "Failed migrations exist!"
    exit 1
fi
```

---

## Workflow Example

### New Contributor Setup

```bash
# 1. Clone and start
git clone <repo>
make docker-dev

# 2. Initialize tracking
make migrate-init

# 3. Check status
make migrate-status-tracked
# Output: Pending: 20240103_add_activity_log_smart_fields

# 4. Apply migrations
make migrate-up-tracked
# Output: Applied 1 migration, recorded in database

# 5. Verify
make migrate-status-tracked
# Output: All migrations applied ✅
```

### Adding New Migration

```bash
# 1. Create migration files
timestamp=$(date +%Y%m%d_%H%M%S)
# Create SQL files

# 2. Test locally
make migrate-up-tracked

# 3. Commit
git add migrations/
git commit -m "feat(database): add new migration"

# 4. Other contributors pull and run
make migrate-up-tracked
# Automatically applies new migration
```

---

## Integration with GORM

### Add to AutoMigrate

```go
// internal/database/db.go
func MigrateDatabase() {
    err := DB.AutoMigrate(
        &models.User{},
        &models.ActivityLog{},
        &models.SocialAccount{},
        &models.SchemaMigration{}, // Add this!
    )
}
```

Now GORM will create the table if it doesn't exist!

---

## Checking Migration Status in Code

```go
package main

import (
    "github.com/gjovanovicst/auth_api/pkg/models"
)

func checkMigrationStatus(db *gorm.DB) {
    // Get all applied migrations
    var migrations []models.SchemaMigration
    db.Where("success = ?", true).
        Order("applied_at DESC").
        Find(&migrations)
    
    for _, m := range migrations {
        fmt.Printf("✅ %s - %s (%v)\n", 
            m.Version, m.Name, m.AppliedAt)
    }
    
    // Check if specific migration applied
    var count int64
    db.Model(&models.SchemaMigration{}).
        Where("version = ? AND success = ?", "20240103_000000", true).
        Count(&count)
    
    if count > 0 {
        fmt.Println("Smart logging migration applied!")
    }
}
```

---

## Troubleshooting

### Table Doesn't Exist

```bash
# Initialize tracking
make migrate-init
```

### Migration Shows as Applied But Wasn't

```bash
# Remove from tracking (BE CAREFUL!)
docker exec auth_db psql -U postgres -d auth_db -c \
  "DELETE FROM schema_migrations WHERE version = '20240103_000000';"

# Then reapply
make migrate-up-tracked
```

### See Failed Migrations

```bash
docker exec auth_db psql -U postgres -d auth_db -c \
  "SELECT * FROM schema_migrations WHERE success = false;"
```

---

## Files Created

1. **migrations/00_create_migrations_table.sql** - Creates tracking table
2. **migrations/00_create_migrations_table_rollback.sql** - Removes tracking table
3. **pkg/models/schema_migration.go** - Go model for tracking table
4. **This file** - Documentation

---

## See Also

- [MIGRATIONS.md](../MIGRATIONS.md) - User migration guide
- [migrations/README.md](../migrations/README.md) - Developer guide
- [MIGRATIONS_DOCKER.md](MIGRATIONS_DOCKER.md) - Docker-specific guide

---

*Migration tracking ensures your database always knows its state* ✅

