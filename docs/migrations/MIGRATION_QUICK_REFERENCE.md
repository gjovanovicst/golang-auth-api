# Migration Quick Reference Card

**One-page guide to understanding the migration system**

---

## 🎯 Two-Tier System

| Tier | Type | When | Purpose |
|------|------|------|---------|
| **1** | GORM AutoMigrate | Automatic (every startup) | Foundation - creates base tables |
| **2** | SQL Migrations | Manual (`make migrate-up`) | Enhancements - complex changes |

---

## 👥 For Different Users

### New Contributor (Never Set Up)

```bash
make docker-dev     # ✅ GORM creates everything automatically
make migrate-up     # ✅ Optional: adds extra enhancements
```

**Result:** Complete database with all features! ✅

---

### Existing Contributor (Has v1.0.0)

```bash
git pull            # Get latest code
make docker-dev     # ✅ GORM adds new fields automatically
make migrate-up     # ✅ Backfills data, adds enhancements
```

**Result:** Upgraded to v1.1.0 with all features! ✅

---

### Production Deployment

```bash
# 1. Test in staging first
# 2. Backup production
make migrate-backup

# 3. Deploy
git pull
docker-compose up -d  # ✅ GORM updates automatically

# 4. Apply manual migrations
make migrate-up

# 5. Verify
make migrate-check
```

**Result:** Production updated, zero downtime! ✅

---

## 🔄 What Happens When

### When You Start App (`make docker-dev`)

```
1. Docker starts PostgreSQL
   ↓
2. Application starts
   ↓
3. GORM AutoMigrate runs automatically
   ↓
4. Checks models vs database
   ↓
5. Adds missing tables/columns
   ↓
6. Application ready!
```

**GORM creates:**
- `users` table
- `activity_logs` table (with smart fields!)
- `social_accounts` table
- `schema_migrations` table

### When You Run `make migrate-up`

```
1. Reads migrations/*.sql files
   ↓
2. Applies each migration
   ↓
3. May show "already exists" warnings (SAFE!)
   ↓
4. Adds comments, indexes, constraints
   ↓
5. Backfills existing data
   ↓
6. Complete!
```

**Adds:**
- Column comments (documentation)
- Optimized indexes (performance)
- Constraints (data integrity)
- Data backfill (for old records)

---

## 📋 When to Use What

### Use GORM AutoMigrate (Automatic) For:

- ✅ New tables
- ✅ New nullable columns
- ✅ Simple indexes
- ✅ Development changes
- ✅ 80% of changes

**Just update the model, restart app!**

### Use SQL Migration (Manual) For:

- ✅ NOT NULL columns
- ✅ Data transformations
- ✅ Renaming columns
- ✅ Complex indexes
- ✅ Breaking changes
- ✅ Production-critical changes

**Create migration file, test, apply!**

---

## ⚡ Quick Commands

```bash
# Daily development
make docker-dev          # Start (GORM runs automatically)

# Check database
make migrate-check       # See schema
make migrate-status      # See tables
make migrate-status-tracked  # See tracked migrations

# Apply migrations
make migrate-up          # Apply SQL migrations
make migrate-down        # Rollback

# Utilities
make migrate-backup      # Backup database
make migrate-test        # Test connection
make migrate-init        # Initialize tracking (first time)
```

---

## ⚠️ Common Questions

### Q: Do I need to run migrations manually?

**New contributors:** No! GORM creates everything.  
**Optional:** Run `make migrate-up` for extras.

**Existing users:** Yes! Run `make migrate-up` to get enhancements.

### Q: What are "already exists" warnings?

**Answer:** SAFE! Means GORM already created those columns from your models. This is EXPECTED and GOOD.

### Q: Will manual migrations break my database?

**Answer:** No! They use `IF NOT EXISTS` clauses. Safe to run multiple times.

### Q: How do I know what's applied?

```bash
make migrate-status-tracked  # See tracked migrations
make migrate-check           # See actual schema
```

---

## 🎯 Current State (v1.1.0)

### What GORM Creates (Automatic)

```
users table
├── id, email, password_hash
└── All user fields

activity_logs table
├── id, user_id, event_type
├── severity, expires_at, is_anomaly  ← Smart logging!
└── All log fields

social_accounts table
└── OAuth data

schema_migrations table
└── Tracking system
```

### What SQL Migration Adds (Manual)

```
Comments on columns
Optimized indexes (GIN, partial, etc.)
Constraints (CHECK, etc.)
Data backfill for existing logs
```

---

## 📊 Migration Flow Diagram

```
Developer makes change
        ↓
    Update model?
        ↓
   ┌────┴────┐
   │         │
Simple?  Complex?
   │         │
   ↓         ↓
GORM    SQL Migration
Auto     Create .sql
   │         │
   └────┬────┘
        ↓
   Test locally
        ↓
   Commit to repo
        ↓
   Other devs pull
        ↓
   make docker-dev (GORM runs)
   make migrate-up (SQL runs)
        ↓
   Everyone has same schema ✅
```

---

## 🚀 Best Practices

### ✅ Do This

- Update models first
- Use GORM for simple changes
- Use SQL for complex changes
- Test migrations locally
- Document everything
- Backup before production

### ❌ Don't Do This

- Skip GORM (it runs automatically)
- Forget to document
- Deploy without testing
- Skip backups
- Ignore "already exists" (it's OK!)

---

## 📚 Full Documentation

| Need | Read |
|------|------|
| **Complete guide** | [MIGRATION_STRATEGY.md](MIGRATION_STRATEGY.md) |
| **User guide** | [MIGRATIONS.md](../MIGRATIONS.md) |
| **Developer guide** | [migrations/README.md](../migrations/README.md) |
| **Docker guide** | [MIGRATIONS_DOCKER.md](MIGRATIONS_DOCKER.md) |
| **Tracking system** | [MIGRATION_TRACKING.md](MIGRATION_TRACKING.md) |

---

## ✅ Summary

**Your system:**
- ✅ GORM AutoMigrate (automatic foundation)
- ✅ SQL Migrations (manual enhancements)
- ✅ Both work together perfectly
- ✅ New contributors get everything automatically
- ✅ Existing users smooth upgrade path
- ✅ Production zero-downtime deployments

**It just works!** 🎉

