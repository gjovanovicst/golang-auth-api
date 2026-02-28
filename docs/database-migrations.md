# Database Migrations

The project uses a two-tier migration system: automatic schema management via GORM and manual SQL migrations for complex changes.

For the full migration system documentation, see [migrations/README.md](migrations/README.md).

---

## Two-Tier System

### 1. GORM AutoMigrate (Automatic)

Runs on application startup:

- Creates tables from Go models
- Adds missing columns
- Creates indexes
- Safe for production
- Cannot handle: column renames, data transformations, complex constraints

### 2. SQL Migrations (Manual)

For complex changes:

- Data transformations
- Column renames and type changes
- Custom indexes and constraints
- Performance optimizations
- Full control with rollback support

---

## Quick Commands

```bash
# Check current migration status
make migrate-status

# Apply all pending migrations
make migrate-up

# Rollback last migration
make migrate-down

# Interactive migration tool (recommended for beginners)
make migrate
```

---

## For New Contributors

```bash
# 1. Start the project (GORM creates base tables automatically)
make docker-dev

# 2. Apply SQL enhancements (optional, but recommended)
make migrate-up

# 3. Start developing
make dev
```

---

## Creating New Migrations

```bash
# 1. Copy the template
cp migrations/TEMPLATE.md migrations/YYYYMMDD_HHMMSS_your_migration.md

# 2. Create forward migration SQL
# migrations/YYYYMMDD_HHMMSS_your_migration.sql

# 3. Create rollback SQL
# migrations/YYYYMMDD_HHMMSS_your_migration_rollback.sql

# 4. Test and apply
make migrate-test
make migrate-up
```

---

## Related Documentation

- [Migration System Overview](migrations/MIGRATIONS.md)
- [User Migration Guide](migrations/USER_GUIDE.md)
- [Upgrade Guide](migrations/UPGRADE_GUIDE.md)
- [Quick Reference](migrations/MIGRATION_QUICK_REFERENCE.md)
- [Docker Migrations](migrations/MIGRATIONS_DOCKER.md)
