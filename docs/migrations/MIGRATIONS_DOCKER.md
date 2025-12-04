# Migrations with Docker

**Quick guide for running migrations when using Docker containers**

---

## 🐳 The Setup

Your database runs in a Docker container named `auth_db`, so migration commands need to execute **inside** the container.

---

## ⚡ Quick Commands

```bash
# Check database is running
docker ps | grep auth_db

# Check what tables exist
make migrate-status

# Apply migrations
make migrate-up

# Check database schema
make migrate-check

# Create backup
make migrate-backup

# Test connection
make migrate-test

# Rollback (if needed)
make migrate-down
```

---

## 📋 All Migration Commands (Docker-Compatible)

### `make migrate-status`
Shows all database tables
```bash
make migrate-status
# Output: Lists users, activity_logs, social_accounts, etc.
```

### `make migrate-up`
Applies pending migrations
```bash
make migrate-up
# Applies: 20240103_add_activity_log_smart_fields.sql
```

**Expected output:**
```
Applying migrations...
Applying: 20240103_add_activity_log_smart_fields.sql
ALTER TABLE
CREATE INDEX
...
✅ Migrations applied successfully!
```

**If you see warnings about "already exists"** - This is **NORMAL and SAFE!** 
It means GORM already created those columns from your models.

### `make migrate-down`
Rolls back the last migration
```bash
make migrate-down
# Rolls back: 20240103_add_activity_log_smart_fields_rollback.sql
```

### `make migrate-check`
Shows detailed database schema
```bash
make migrate-check
# Shows: Table structure, columns, indexes
```

### `make migrate-backup`
Creates a database backup
```bash
make migrate-backup
# Creates: backups/backup_YYYYMMDD_HHMMSS.sql
```

### `make migrate-test`
Tests database connection
```bash
make migrate-test
# ✅ Connection successful!
```

### `make migrate-list`
Lists available migration files
```bash
make migrate-list
# Shows: All .sql files in migrations/
```

---

## 🔧 Manual Docker Commands

If you want to run SQL commands manually:

### Connect to Database Container
```bash
docker exec -it auth_db psql -U postgres -d auth_db
```

### Run SQL File
```bash
docker exec -i auth_db psql -U postgres -d auth_db < migrations/your_migration.sql
```

### Check Tables
```bash
docker exec auth_db psql -U postgres -d auth_db -c "\dt"
```

### Check Table Structure
```bash
docker exec auth_db psql -U postgres -d auth_db -c "\d activity_logs"
```

### Run SQL Query
```bash
docker exec auth_db psql -U postgres -d auth_db -c "SELECT COUNT(*) FROM activity_logs;"
```

---

## 🆚 Docker vs Local PostgreSQL

### If Database is in Docker (Your Setup)
✅ **Use:** `make migrate-*` commands (now Docker-aware)  
✅ **Use:** `docker exec` commands  
❌ **Don't use:** `psql` directly on host  
❌ **Don't use:** `scripts/migrate.sh` (requires local psql)

### If Database is Local (Not Your Setup)
- Can use `psql` directly
- Can use `scripts/migrate.sh`
- No Docker commands needed

---

## 📊 Complete First-Time Setup

```bash
# 1. Start everything
make docker-dev
# ✅ PostgreSQL starts
# ✅ Redis starts  
# ✅ Application starts
# ✅ GORM creates all tables automatically

# 2. Check tables were created
make migrate-status
# Should show: users, activity_logs, social_accounts

# 3. Apply additional migrations (adds enhancements)
make migrate-up
# ✅ Adds column comments
# ✅ Adds optimized indexes
# ✅ May show "already exists" warnings (this is OK!)

# 4. Verify schema
make migrate-check
# Shows complete activity_logs structure with:
# - severity, expires_at, is_anomaly fields
# - All indexes
# - All constraints
```

---

## ⚠️ Troubleshooting

### Error: "Cannot connect to database"

**Check if container is running:**
```bash
docker ps | grep auth_db
```

**If not running:**
```bash
make docker-dev
```

### Error: "psql command not found"

**This means you're trying to use local psql.**

**Solution:** Use Docker-aware commands:
```bash
# ❌ Wrong: psql -U postgres -d auth_db
# ✅ Right: docker exec auth_db psql -U postgres -d auth_db

# OR just use make commands:
make migrate-up
make migrate-status
```

### Warning: "column already exists"

**This is NORMAL!**
- ✅ Your models already have these fields
- ✅ GORM created them automatically
- ✅ Migration is idempotent (safe to run)
- ✅ No action needed

### Error: "No such container: auth_db"

**Container name might be different:**
```bash
# Check actual container name
docker ps

# Use correct name in commands
docker exec -it YOUR_CONTAINER_NAME psql -U postgres -d auth_db
```

---

## 🎯 Common Tasks

### Check What Tables Exist
```bash
make migrate-status
```

### Check if Smart Logging Fields Exist
```bash
make migrate-check
# Look for: severity, expires_at, is_anomaly
```

### Apply Migrations
```bash
make migrate-up
```

### Create Backup Before Changes
```bash
make migrate-backup
```

### Restore from Backup
```bash
docker exec -i auth_db psql -U postgres -d auth_db < backups/backup_YYYYMMDD_HHMMSS.sql
```

### Delete All Tables (Clean Start)
```bash
# ⚠️ WARNING: Destroys all data!
docker-compose down -v
make docker-dev
```

---

## 📚 See Also

- [MIGRATIONS.md](../MIGRATIONS.md) - Complete migration guide
- [README.md](../README.md) - Project setup
- [migrations/README.md](../migrations/README.md) - Developer guide

---

## 💡 Pro Tips

### Tip 1: Use Make Commands
```bash
# Easier to remember
make migrate-up

# Instead of
docker exec -i auth_db psql -U postgres -d auth_db < migrations/file.sql
```

### Tip 2: Check Before Migrating
```bash
make migrate-status    # See current state
make migrate-backup    # Backup first
make migrate-up        # Apply changes
make migrate-check     # Verify results
```

### Tip 3: Database Logs
```bash
# If migration fails, check database logs
docker logs auth_db
```

### Tip 4: Application Logs
```bash
# Check if GORM migrations ran
docker logs auth_api_dev | grep -i migrate
```

---

## ✅ Summary

**Your migrations now work with Docker!**

```bash
make migrate-up      # ✅ Works with Docker
make migrate-status  # ✅ Works with Docker
make migrate-check   # ✅ Works with Docker
make migrate-backup  # ✅ Works with Docker
```

All commands automatically execute inside the `auth_db` container. No need to install PostgreSQL locally! 🎉

