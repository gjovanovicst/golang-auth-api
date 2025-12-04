# Database Migrations

This project uses a two-tier migration system for database schema management.

---

## Quick Start

```bash
# Start development (creates all tables automatically)
make docker-dev

# Apply additional enhancements (optional)
make migrate-up

# Check database status
make migrate-check
```

---

## Two-Tier System

### 1. GORM AutoMigrate (Automatic)
- Runs on every app startup
- Creates tables from Go models
- Adds new columns safely
- **No manual action needed**

### 2. SQL Migrations (Manual)
- Complex changes requiring control
- Data transformations
- Performance optimizations
- Run via `make migrate-up`

---

## Documentation

### For Users
- **[User Guide](docs/migrations/USER_GUIDE.md)** - How to run migrations
- **[Upgrade Guide](docs/migrations/UPGRADE_GUIDE.md)** - Version upgrades
- **[Docker Guide](docs/MIGRATIONS_DOCKER.md)** - Docker-specific commands

### For Contributors
- **[Developer Guide](migrations/README.md)** - Creating migrations
- **[Migration Template](migrations/TEMPLATE.md)** - Template for new migrations
- **[Strategy Guide](docs/MIGRATION_STRATEGY.md)** - Complete strategy

### Reference
- **[Migration Tracking](docs/MIGRATION_TRACKING.md)** - Tracking system
- **[AutoMigrate in Production](docs/AUTOMIGRATE_PRODUCTION.md)** - Production considerations
- **[Quick Reference](docs/MIGRATION_QUICK_REFERENCE.md)** - One-page reference

---

## Common Commands

```bash
# Migration commands
make migrate-status          # Show database tables
make migrate-up              # Apply migrations
make migrate-down            # Rollback migration
make migrate-check           # Check schema
make migrate-backup          # Create backup

# Migration tracking
make migrate-init            # Initialize tracking (first time)
make migrate-status-tracked  # Show tracked migrations
```

---

## Need Help?

- Read [docs/migrations/USER_GUIDE.md](docs/migrations/USER_GUIDE.md) for detailed instructions
- Check [BREAKING_CHANGES.md](BREAKING_CHANGES.md) before upgrading
- See [docs/MIGRATION_QUICK_REFERENCE.md](docs/MIGRATION_QUICK_REFERENCE.md) for quick reference

