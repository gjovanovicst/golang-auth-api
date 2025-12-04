# Migration System Quick Start

**5-minute guide to get started with database migrations**

---

## 🚀 For Users: Running Migrations

### Option 1: Interactive Tool (Easiest)

```bash
make migrate
```

Choose from menu:
1. Show migration status
2. Apply migrations
3. Rollback
4. List available
5. Backup database
6. Test connection

### Option 2: Quick Commands

```bash
# Check what needs to be migrated
make migrate-status

# Backup first (always!)
make migrate-backup

# Apply all pending migrations
make migrate-up

# If something goes wrong
make migrate-down  # Rollback
```

### Option 3: Manual

```bash
# Backup
pg_dump -U postgres -d auth_db > backup.sql

# Apply
psql -U postgres -d auth_db -f migrations/YYYYMMDD_migration.sql

# Verify
psql -U postgres -d auth_db -c "\d table_name"
```

---

## 📖 For Users: Upgrading Versions

### Step 1: Check What's New

```bash
# See what changed
cat BREAKING_CHANGES.md

# Read upgrade instructions
cat UPGRADE_GUIDE.md
```

### Step 2: Upgrade

```bash
# Development (automatic)
git pull
make docker-dev

# Production
git pull
git checkout v1.1.0
make migrate-backup
make migrate-up
docker-compose up -d
```

### Step 3: Verify

```bash
# Check app logs
docker logs auth_api_dev

# Check migration status
make migrate-status
```

---

## 🛠️ For Contributors: Creating Migrations

### Step 1: Create Files

```bash
# Get timestamp
timestamp=$(date +%Y%m%d_%H%M%S)

# Copy template
cp migrations/TEMPLATE.md migrations/${timestamp}_your_change.md

# Create SQL files
touch migrations/${timestamp}_your_change.sql
touch migrations/${timestamp}_your_change_rollback.sql
```

### Step 2: Write Migration

**Forward (migrations/YYYYMMDD_your_change.sql):**
```sql
BEGIN;
ALTER TABLE users ADD COLUMN preferences JSONB DEFAULT '{}';
CREATE INDEX idx_users_preferences ON users USING GIN (preferences);
COMMIT;
```

**Rollback (migrations/YYYYMMDD_your_change_rollback.sql):**
```sql
BEGIN;
DROP INDEX IF EXISTS idx_users_preferences;
ALTER TABLE users DROP COLUMN IF EXISTS preferences;
COMMIT;
```

### Step 3: Test

```bash
# Apply
psql -U postgres -d auth_db_test -f migrations/YYYYMMDD_your_change.sql

# Verify
psql -U postgres -d auth_db_test -c "\d users"

# Rollback
psql -U postgres -d auth_db_test -f migrations/YYYYMMDD_your_change_rollback.sql

# Verify rollback
psql -U postgres -d auth_db_test -c "\d users"
```

### Step 4: Document

Fill out the `.md` template with:
- What changed
- Why it changed
- Impact assessment
- Breaking changes (if any)

### Step 5: Update Docs

- [ ] `migrations/MIGRATIONS_LOG.md` - Add entry
- [ ] `BREAKING_CHANGES.md` - If breaking
- [ ] `UPGRADE_GUIDE.md` - If new version
- [ ] `CHANGELOG.md` - Add to changelog

### Step 6: Submit PR

```bash
git add migrations/
git commit -m "feat(database): add user preferences support"
git push origin feature/user-preferences
```

Add "migration" label to PR.

---

## 📊 Quick Reference

### When to Create Migration?

| Change | SQL Migration | GORM AutoMigrate |
|--------|---------------|------------------|
| Add nullable column | ✅ Either | ✅ Preferred |
| Add NOT NULL column | ✅ Required | ❌ No |
| Change column type | ✅ Required | ❌ No |
| Remove column | ✅ Required | ❌ No |
| Rename column | ✅ Required | ❌ No |
| Add table | ✅ Either | ✅ Preferred |
| Transform data | ✅ Required | ❌ No |
| Breaking change | ✅ Required | ❌ No |

### Key Commands

| Task | Command |
|------|---------|
| Interactive tool | `make migrate` |
| Check status | `make migrate-status` |
| Apply migrations | `make migrate-up` |
| Rollback | `make migrate-down` |
| List migrations | `make migrate-list` |
| Backup database | `make migrate-backup` |
| Test connection | `make migrate-test` |

### Key Files

| Need | Read |
|------|------|
| How to run migrations | [MIGRATIONS.md](../MIGRATIONS.md) |
| How to upgrade | [UPGRADE_GUIDE.md](../UPGRADE_GUIDE.md) |
| What breaks | [BREAKING_CHANGES.md](../BREAKING_CHANGES.md) |
| How to create | [migrations/README.md](../migrations/README.md) |
| Template | [migrations/TEMPLATE.md](../migrations/TEMPLATE.md) |
| History | [migrations/MIGRATIONS_LOG.md](../migrations/MIGRATIONS_LOG.md) |

---

## ⚠️ Important Rules

### Always Do
- ✅ Backup before migrating
- ✅ Test in development first
- ✅ Use transactions (BEGIN/COMMIT)
- ✅ Make migrations idempotent (IF EXISTS)
- ✅ Provide rollback scripts
- ✅ Document everything

### Never Do
- ❌ Skip backups
- ❌ Test in production first
- ❌ Forget rollback script
- ❌ Make non-idempotent migrations
- ❌ Skip documentation
- ❌ Commit breaking changes without docs

---

## 🆘 Troubleshooting

### "Migration failed"

```bash
# 1. Check the error message
docker logs auth_db

# 2. Rollback
make migrate-down

# 3. Fix the SQL
# Edit migrations/YYYYMMDD_migration.sql

# 4. Try again
make migrate-up
```

### "Column already exists"

```sql
-- Use IF NOT EXISTS
ALTER TABLE users ADD COLUMN IF NOT EXISTS new_field VARCHAR(100);
```

### "Can't connect to database"

```bash
# Check database is running
docker ps | grep postgres

# Test connection
make migrate-test

# Check environment variables
cat .env | grep DB_
```

---

## 📚 Need More Help?

- 📖 **Full guide:** [MIGRATIONS.md](../MIGRATIONS.md)
- 🔧 **Developer guide:** [migrations/README.md](../migrations/README.md)
- 🚀 **Upgrade guide:** [UPGRADE_GUIDE.md](../UPGRADE_GUIDE.md)
- 💔 **Breaking changes:** [BREAKING_CHANGES.md](../BREAKING_CHANGES.md)
- 🤝 **Contributing:** [CONTRIBUTING.md](../CONTRIBUTING.md)

---

## ✅ Checklist for New Migration

Before submitting PR:

- [ ] Created forward migration SQL
- [ ] Created rollback migration SQL
- [ ] Created documentation file
- [ ] Tested apply locally
- [ ] Tested rollback locally
- [ ] Tested with realistic data
- [ ] Updated MIGRATIONS_LOG.md
- [ ] Updated BREAKING_CHANGES.md (if breaking)
- [ ] Updated UPGRADE_GUIDE.md (if version change)
- [ ] Updated CHANGELOG.md
- [ ] Added tests
- [ ] PR labeled "migration"

---

*Quick start guide - For full details see linked documentation*

