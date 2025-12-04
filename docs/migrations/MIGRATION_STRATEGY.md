# Migration Strategy - Complete Guide

**Understanding the two-tier migration system and how to use it**

---

## 🎯 The Two-Tier System Explained

Your project uses **BOTH** automatic and manual migrations working together:

### Tier 1: GORM AutoMigrate (Automatic - Foundation)
**When:** Every time the application starts  
**What:** Creates base tables from Go models  
**Purpose:** Foundation - ensures core tables exist  

### Tier 2: Manual SQL Migrations (Manual - Enhancements)
**When:** Run explicitly via `make migrate-up`  
**What:** Complex changes, data transformations, optimizations  
**Purpose:** Enhancement - adds features GORM can't handle  

---

## 📊 Current State of Your Project

### What Was Done BEFORE Migration System

**Phase 1: Initial Development (Past)**
```
App starts → GORM AutoMigrate → Creates all tables
```

**Tables created:**
- `users` (from User model)
- `activity_logs` (from ActivityLog model - initially basic)
- `social_accounts` (from SocialAccount model)

**Everyone got these automatically!**

### What Was Added WITH Migration System

**Phase 2: Smart Logging Added (v1.1.0)**

**Step 1:** Updated `models.ActivityLog` to include:
```go
type ActivityLog struct {
    // ... existing fields
    Severity  EventSeverity   `gorm:"..."`  // NEW
    ExpiresAt *time.Time      `gorm:"..."`  // NEW
    IsAnomaly bool            `gorm:"..."`  // NEW
}
```

**Step 2:** Created SQL migration:
- `migrations/20240103_add_activity_log_smart_fields.sql`
- Adds columns (with `IF NOT EXISTS` - safe!)
- Adds comments
- Adds optimized indexes
- Backfills data for existing logs

---

## 🔄 Migration Flow for Different Scenarios

### Scenario 1: Brand New Contributor (Never Set Up Before)

**What they do:**
```bash
# 1. Clone repository
git clone <repo>
cd auth_api/v1.0.0

# 2. Start application
make docker-dev
```

**What happens automatically:**
1. ✅ Docker starts PostgreSQL (empty database)
2. ✅ Application starts
3. ✅ **GORM AutoMigrate runs**
4. ✅ Creates ALL tables including `activity_logs` with smart fields already!

**Why?** Because the model (`models.ActivityLog`) already has `Severity`, `ExpiresAt`, `IsAnomaly` fields!

**Do they need manual migration?**
```bash
# Optional (adds extra enhancements):
make migrate-up
```

**Result:** Some "already exists" warnings (SAFE - means GORM already created them)

**What manual migration adds:**
- ✅ Column comments (documentation)
- ✅ Optimized indexes
- ✅ Constraints
- ⚠️ Warnings about "already exists" (expected!)

---

### Scenario 2: Existing Contributor (Has Old Setup from v1.0.0)

**Their situation:**
```
Database from v1.0.0:
├── users ✅
└── activity_logs ⚠️ (OLD schema - missing smart fields)
    ├── id, user_id, event_type, timestamp ✅
    └── Missing: severity, expires_at, is_anomaly ❌
```

**What they do:**
```bash
# 1. Pull latest code
git pull origin main

# 2. Restart application
make docker-dev
```

**What happens:**
1. ✅ GORM AutoMigrate runs
2. ⚠️ GORM sees new fields in model
3. ✅ **GORM adds missing columns** (severity, expires_at, is_anomaly)
4. ✅ Application starts with new schema

**Then run manual migration:**
```bash
make migrate-up
```

**What manual migration does:**
- ✅ Ensures columns exist (shows "already exists" - OK!)
- ✅ Adds column comments
- ✅ Adds optimized indexes
- ✅ **Backfills data** - Sets severity for existing 1,000+ logs
- ✅ Sets expiration dates based on event type

**Result:** Database fully upgraded with all enhancements!

---

### Scenario 3: Future New Features

**Example: Adding user preferences**

**Developer creates:**

**Step 1: Update Model**
```go
// pkg/models/user.go
type User struct {
    // ... existing fields
    Preferences JSONB `gorm:"type:jsonb;default:'{}'" json:"preferences"` // NEW
}
```

**Step 2: Decide Migration Approach**

**Option A: Simple - Let GORM Handle It**
```bash
# Just restart app
make docker-dev

# GORM adds the new column automatically
# Everyone gets it on next startup
```

**Use when:**
- ✅ Adding nullable column
- ✅ Simple change
- ✅ No data transformation needed

**Option B: Complex - Create SQL Migration**
```bash
# Create migration file
migrations/20240110_150000_add_user_preferences.sql
```

```sql
BEGIN;

-- Add column
ALTER TABLE users ADD COLUMN IF NOT EXISTS preferences JSONB DEFAULT '{}';

-- Create index
CREATE INDEX IF NOT EXISTS idx_users_preferences ON users USING GIN (preferences);

-- Migrate existing data
UPDATE users SET preferences = '{"theme":"dark"}' WHERE preferences IS NULL;

COMMIT;
```

**Use when:**
- ✅ NOT NULL column (need default first)
- ✅ Data transformation required
- ✅ Complex constraints
- ✅ Performance-critical indexes

---

## 🏭 Production Deployment Strategy

### Pre-Deployment Checklist

**Before deploying to production:**

1. **Test in staging first!**
   ```bash
   # In staging environment
   git pull
   make docker-dev
   make migrate-up
   # Test thoroughly!
   ```

2. **Check breaking changes:**
   ```bash
   cat BREAKING_CHANGES.md
   # Any breaking changes?
   ```

3. **Plan downtime if needed:**
   - Most migrations: ✅ Zero downtime
   - Breaking changes: ⚠️ May need downtime

### Production Deployment Flow

**Option A: Zero-Downtime Deployment (Recommended)**

```bash
# 1. Backup production database
make migrate-backup
# Or: pg_dump production > backup_$(date +%Y%m%d).sql

# 2. Deploy new code (keep app running)
git pull
docker-compose build

# 3. GORM AutoMigrate will run on restart
docker-compose up -d

# App restarts:
# - GORM adds any new columns (backward compatible)
# - Application runs with new code

# 4. Apply manual migrations (if any)
make migrate-up

# 5. Verify
make migrate-check
curl https://your-api.com/health
```

**Why this works:**
- GORM adds columns as NULL (safe)
- Old code ignores new columns
- New code uses new columns
- Manual migration enhances
- No downtime! ✅

**Option B: With Maintenance Window**

```bash
# 1. Enable maintenance mode
# (Return 503 to all requests)

# 2. Backup database
make migrate-backup

# 3. Stop application
docker-compose down

# 4. Apply migrations
make migrate-up

# 5. Deploy new code
git pull
docker-compose up -d

# 6. Verify
make migrate-check

# 7. Disable maintenance mode
```

---

## 📋 Decision Matrix: When to Use What

| Change Type | GORM AutoMigrate | Manual SQL Migration |
|-------------|------------------|----------------------|
| Add new table | ✅ Yes | Optional |
| Add nullable column | ✅ Yes | Optional (for comments/indexes) |
| Add NOT NULL column | ❌ No | ✅ Yes (set default first) |
| Add index (simple) | ✅ Yes | Optional |
| Add index (complex/GIN/partial) | ❌ No | ✅ Yes |
| Modify column type | ❌ No | ✅ Yes |
| Rename column | ❌ No | ✅ Yes |
| Remove column | ❌ No | ✅ Yes |
| Transform data | ❌ No | ✅ Yes |
| Add constraints | ⚠️ Limited | ✅ Yes |
| Backfill data | ❌ No | ✅ Yes |

---

## 🎓 Best Practices Going Forward

### For New Features

**1. Start with the Model**
```go
// Always update the Go model first
type User struct {
    NewField string `gorm:"type:varchar(100)" json:"new_field"`
}
```

**2. Decide if SQL Migration Needed**

**Need SQL migration if:**
- Data transformation required
- NOT NULL column
- Complex indexes
- Breaking change
- Production has existing data

**Don't need SQL migration if:**
- Simple nullable column
- New table
- GORM can handle it

**3. Document Everything**
- Update `migrations/MIGRATIONS_LOG.md`
- Update `CHANGELOG.md`
- If breaking: Update `BREAKING_CHANGES.md`

### For Contributors

**New contributor setup:**
```bash
make docker-dev     # GORM creates everything
make migrate-up     # Optional enhancements
```

**Existing contributor update:**
```bash
git pull            # Get latest code
make docker-dev     # GORM adds new fields
make migrate-up     # Apply any new migrations
```

**Both end up with same schema!** ✅

---

## 🔍 How to Check Current State

### Check What GORM Will Create

```go
// Look at models in pkg/models/
// These are created automatically:
- User
- ActivityLog
- SocialAccount
- SchemaMigration
```

### Check What Manual Migrations Exist

```bash
ls -la migrations/*.sql

# Currently:
# - 00_create_migrations_table.sql (tracking system)
# - 20240103_add_activity_log_smart_fields.sql (smart logging)
```

### Check Database State

```bash
make migrate-check          # See actual database
make migrate-status-tracked # See tracked migrations
```

---

## 🚀 Recommended Workflow

### For Development

```bash
# Daily development
make docker-dev
# Changes auto-reload
# GORM keeps schema up to date
```

### For Adding Database Changes

**Simple changes:**
```bash
# 1. Update model in pkg/models/
# 2. Restart: make docker-dev
# 3. GORM adds it automatically
# 4. Commit model change
```

**Complex changes:**
```bash
# 1. Update model (if needed)
# 2. Create SQL migration
# 3. Test: make migrate-up
# 4. Test rollback: make migrate-down
# 5. Document in MIGRATIONS_LOG.md
# 6. Commit model + migration files
```

### For Production

```bash
# Weekly/monthly deployments
# 1. Test in staging
# 2. Backup production
# 3. Deploy new code (GORM runs automatically)
# 4. Apply manual migrations (if any)
# 5. Verify
```

---

## 💡 Key Principles

### 1. Models are Source of Truth
```
Go Model → GORM AutoMigrate → Database Schema
```
Always update models first!

### 2. GORM Handles 80% of Changes
Most changes can be automatic:
- New tables
- New nullable columns
- Simple indexes

### 3. SQL Migrations for Complex Cases
Use manual migrations for:
- Data transformations
- NOT NULL columns
- Breaking changes

### 4. Both Work Together
```
GORM (foundation) + SQL migrations (enhancements) = Complete schema
```

### 5. Always Backward Compatible
- Add columns as NULL first
- Transform data separately
- Make NOT NULL later (if needed)

---

## 🎯 Summary

### Current System (v1.1.0)

**Automatic (GORM):**
- ✅ Creates all base tables
- ✅ Adds `activity_logs` with smart fields
- ✅ Runs on every startup
- ✅ Safe and backward compatible

**Manual (SQL):**
- ✅ One migration: Smart logging enhancements
- ✅ Adds comments, indexes, constraints
- ✅ Backfills data for existing logs
- ✅ Run via: `make migrate-up`

**Result:**
- ✅ New contributors: Full schema automatically
- ✅ Existing contributors: Smooth upgrade path
- ✅ Production: Zero-downtime deployments
- ✅ Everyone ends up with same schema

### Going Forward

**For new features:**
1. Update Go model
2. Decide if SQL migration needed
3. Test locally
4. Document
5. Deploy

**For contributors:**
1. `git pull`
2. `make docker-dev` (GORM updates automatically)
3. `make migrate-up` (if new migrations exist)

**For production:**
1. Test in staging
2. Backup
3. Deploy (GORM runs automatically)
4. Apply manual migrations
5. Verify

---

## 📚 Related Documentation

- [MIGRATIONS.md](../MIGRATIONS.md) - User migration guide
- [migrations/README.md](../migrations/README.md) - Developer guide
- [UPGRADE_GUIDE.md](../UPGRADE_GUIDE.md) - Version upgrades
- [MIGRATION_TRACKING.md](MIGRATION_TRACKING.md) - Tracking system

---

**Your two-tier system gives you the best of both worlds:**
- ✅ Automatic ease for simple changes (GORM)
- ✅ Manual control for complex changes (SQL)
- ✅ Zero-downtime production deployments
- ✅ Smooth onboarding for new contributors

🎉 **This is actually a very solid approach used by many production systems!**

