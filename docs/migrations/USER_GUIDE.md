# Database Migrations Guide

## Overview

This project uses a hybrid migration approach combining **GORM AutoMigrate** for seamless schema updates and **SQL migrations** for complex changes requiring precise control.

---

## Quick Start

### Running Migrations

**Automatic (Recommended for Development):**
```bash
# Start the application - migrations run automatically
make docker-dev
# OR
go run cmd/api/main.go
```

**Manual (Recommended for Production):**
```bash
# Apply specific SQL migration
psql -U postgres -d auth_db -f migrations/YYYYMMDD_migration_name.sql

# OR use the migration script
./scripts/migrate.sh

# Windows
scripts\migrate.bat
```

---

## Migration Types

### 1. GORM AutoMigrate (Automatic)

**Used for:**
- Adding new tables
- Adding new columns
- Adding indexes
- Non-breaking schema changes

**How it works:**
- Runs automatically on application startup
- Defined in `internal/database/db.go` → `MigrateDatabase()`
- Based on model definitions in `pkg/models/`

**Example:**
```go
// Add new field to model
type User struct {
    // ... existing fields
    NewField string `gorm:"type:varchar(100)" json:"new_field"`
}

// GORM will automatically add the column on next startup
```

**Limitations:**
- Cannot delete columns
- Cannot change column types
- Cannot add complex constraints
- Cannot perform data transformations

### 2. SQL Migrations (Manual)

**Used for:**
- Complex schema changes
- Data transformations
- Breaking changes
- Precise control needed

**Location:** `migrations/` directory

**Naming Convention:**
```
YYYYMMDD_HHMMSS_description.sql         # Forward migration
YYYYMMDD_HHMMSS_description_rollback.sql # Rollback
YYYYMMDD_HHMMSS_description.md          # Documentation
```

---

## Current Migrations

### Applied Migrations

| Date | Migration | Type | Breaking | Status |
|------|-----------|------|----------|--------|
| 2024-01-03 | Smart Activity Logging | SQL | No | ✅ Applied |

See [MIGRATIONS_LOG.md](migrations/MIGRATIONS_LOG.md) for complete history.

---

## How to Apply Migrations

### Development Environment

**Option 1: Automatic (Easiest)**
```bash
# Just start the app - GORM AutoMigrate runs
make docker-dev
```

**Option 2: Interactive Script**
```bash
# Unix/Mac
./scripts/migrate.sh

# Windows
scripts\migrate.bat
```

**Option 3: Make Commands**
```bash
# Apply all pending migrations
make migrate-up

# Rollback last migration
make migrate-down

# Check migration status
make migrate-status
```

### Production Environment

**Step 1: Backup Database**
```bash
# Always backup first!
pg_dump -U postgres -d auth_db > backup_$(date +%Y%m%d_%H%M%S).sql
```

**Step 2: Apply Migration**
```bash
# Test in staging first
psql -U postgres -d auth_db_staging -f migrations/YYYYMMDD_migration.sql

# Then production
psql -U postgres -d auth_db_production -f migrations/YYYYMMDD_migration.sql
```

**Step 3: Verify**
```bash
# Check migration succeeded
psql -U postgres -d auth_db -c "\d table_name"
```

**Step 4: Update Application**
```bash
# Deploy new version
docker-compose up -d
```

---

## Rollback Process

### If Migration Fails

**1. Stop the Application**
```bash
docker-compose down
```

**2. Apply Rollback**
```bash
psql -U postgres -d auth_db -f migrations/YYYYMMDD_migration_rollback.sql
```

**3. Restore from Backup (if needed)**
```bash
psql -U postgres -d auth_db < backup_YYYYMMDD_HHMMSS.sql
```

**4. Restart with Previous Version**
```bash
git checkout previous-version
docker-compose up -d
```

---

## Creating New Migrations

### When to Use SQL Migrations

Create SQL migration when:
- ✅ Breaking schema changes
- ✅ Data transformations needed
- ✅ Complex constraints
- ✅ Renaming columns/tables
- ✅ Changing column types
- ✅ Production deployment

### When to Use AutoMigrate

Use AutoMigrate when:
- ✅ Adding new nullable columns
- ✅ Adding new tables
- ✅ Adding indexes
- ✅ Development/testing
- ✅ Non-breaking changes

### Creating SQL Migration

**1. Use Template**
```bash
cp migrations/TEMPLATE.md migrations/$(date +%Y%m%d_%H%M%S)_your_migration.md
```

**2. Create SQL Files**
```sql
-- migrations/20240103_120000_your_migration.sql
-- Forward migration
ALTER TABLE users ADD COLUMN new_field VARCHAR(100);
```

```sql
-- migrations/20240103_120000_your_migration_rollback.sql
-- Rollback migration
ALTER TABLE users DROP COLUMN new_field;
```

**3. Document It**
See [TEMPLATE.md](migrations/TEMPLATE.md) for documentation template.

**4. Test Locally**
```bash
# Apply
psql -U postgres -d auth_db_test -f migrations/20240103_120000_your_migration.sql

# Verify
psql -U postgres -d auth_db_test -c "\d users"

# Rollback
psql -U postgres -d auth_db_test -f migrations/20240103_120000_your_migration_rollback.sql

# Verify rollback
psql -U postgres -d auth_db_test -c "\d users"
```

**5. Update Documentation**
- Add to [MIGRATIONS_LOG.md](migrations/MIGRATIONS_LOG.md)
- Update [BREAKING_CHANGES.md](BREAKING_CHANGES.md) if breaking
- Update [CHANGELOG.md](CHANGELOG.md)

---

## Migration Checklist

Before creating a migration:
- [ ] Determine if SQL or AutoMigrate is appropriate
- [ ] Create migration files (up + down)
- [ ] Write migration documentation
- [ ] Test migration locally
- [ ] Test rollback locally
- [ ] Update MIGRATIONS_LOG.md
- [ ] Update BREAKING_CHANGES.md (if breaking)
- [ ] Update CHANGELOG.md
- [ ] Create PR with migration

---

## Troubleshooting

### Migration Failed

**Error: "relation already exists"**
```bash
# Check if table exists
psql -U postgres -d auth_db -c "\dt"

# If exists, skip or modify migration
```

**Error: "column already exists"**
```bash
# Check existing columns
psql -U postgres -d auth_db -c "\d table_name"

# Add IF NOT EXISTS clause
ALTER TABLE users ADD COLUMN IF NOT EXISTS new_field VARCHAR(100);
```

**Error: "constraint violation"**
```bash
# Check existing data
SELECT * FROM table_name WHERE problematic_condition;

# Fix data first, then apply migration
```

### GORM AutoMigrate Not Working

**Issue: Column not added**
```bash
# Check GORM logs
docker logs auth_api_dev | grep "ALTER TABLE"

# Verify model has correct tags
type User struct {
    NewField string `gorm:"type:varchar(100)" json:"new_field"`  // ✅
}
```

**Issue: Index not created**
```bash
# Check indexes
psql -U postgres -d auth_db -c "\di"

# Verify index tag
Field string `gorm:"index" json:"field"`  // ✅
```

---

## Best Practices

### 1. Always Backup First
```bash
pg_dump -U postgres -d auth_db > backup_$(date +%Y%m%d_%H%M%S).sql
```

### 2. Test in Development
```bash
# Test full cycle: apply → verify → rollback → verify
```

### 3. Use Transactions (SQL Migrations)
```sql
BEGIN;
  -- Your migration here
  ALTER TABLE users ADD COLUMN new_field VARCHAR(100);
COMMIT;
-- Add ROLLBACK if something fails
```

### 4. Make Migrations Reversible
Always provide rollback scripts!

### 5. Document Everything
Include:
- What changed
- Why it changed
- How to rollback
- Impact on application

### 6. Gradual Migrations for Breaking Changes
```sql
-- Step 1: Add new nullable column
ALTER TABLE users ADD COLUMN new_email VARCHAR(255);

-- Step 2: Migrate data (in code or separate script)
-- UPDATE users SET new_email = old_email;

-- Step 3: Make not null (after verification)
-- ALTER TABLE users ALTER COLUMN new_email SET NOT NULL;

-- Step 4: Drop old column (in next release)
-- ALTER TABLE users DROP COLUMN old_email;
```

---

## Version Compatibility

| App Version | Min DB Version | Migrations Required |
|-------------|----------------|---------------------|
| v1.0.0 | v1.0.0 | None |
| v1.1.0 | v1.0.0 | Smart Logging (2024-01-03) |

See [UPGRADE_GUIDE.md](UPGRADE_GUIDE.md) for detailed upgrade instructions.

---

## See Also

- [BREAKING_CHANGES.md](BREAKING_CHANGES.md) - Breaking changes tracker
- [UPGRADE_GUIDE.md](UPGRADE_GUIDE.md) - Version upgrade guide
- [migrations/README.md](migrations/README.md) - Developer guide
- [migrations/TEMPLATE.md](migrations/TEMPLATE.md) - Migration template
- [CONTRIBUTING.md](CONTRIBUTING.md) - Contributor guidelines

---

## Need Help?

- 📖 Read [migrations/README.md](migrations/README.md) for detailed developer guide
- 🐛 Check [Troubleshooting](#troubleshooting) section above
- 💬 Open an issue on GitHub
- 📧 Contact maintainers

