# Migrations Directory

This directory contains all database migration files for the Authentication API.

---

## Quick Reference

| What | Command |
|------|---------|
| **Apply migrations** | `make migrate-up` |
| **Check status** | `make migrate-status` |
| **Rollback** | `make migrate-down` |
| **Interactive tool** | `make migrate` or `./scripts/migrate.sh` |

---

## Directory Structure

```
migrations/
├── README.md                                      # This file
├── TEMPLATE.md                                     # Migration template
├── MIGRATIONS_LOG.md                               # Applied migrations log
├── 20240103_add_activity_log_smart_fields.sql     # Forward migration
├── 20240103_add_activity_log_smart_fields_rollback.sql  # Rollback
└── 20240103_add_activity_log_smart_fields.md      # Documentation
```

---

## File Naming Convention

**Format:**
```
YYYYMMDD_HHMMSS_description.sql         # Forward migration
YYYYMMDD_HHMMSS_description_rollback.sql # Rollback migration
YYYYMMDD_HHMMSS_description.md          # Documentation
```

**Examples:**
```
20240103_120000_add_user_preferences.sql
20240103_120000_add_user_preferences_rollback.sql
20240103_120000_add_user_preferences.md
```

**Rules:**
- Use timestamp: `YYYYMMDD_HHMMSS` (24-hour format)
- Use snake_case for description
- Keep description short but descriptive
- Always create both forward and rollback files
- Always create documentation file

---

## Creating a New Migration

### Step 1: Copy Template

```bash
# Get current timestamp
timestamp=$(date +%Y%m%d_%H%M%S)

# Copy template
cp migrations/TEMPLATE.md migrations/${timestamp}_your_description.md
```

### Step 2: Create SQL Files

**Forward Migration:**
```sql
-- migrations/YYYYMMDD_HHMMSS_your_description.sql

BEGIN;

-- Your changes here
ALTER TABLE table_name ADD COLUMN new_column VARCHAR(100);

COMMIT;
```

**Rollback Migration:**
```sql
-- migrations/YYYYMMDD_HHMMSS_your_description_rollback.sql

BEGIN;

-- Reverse your changes
ALTER TABLE table_name DROP COLUMN new_column;

COMMIT;
```

### Step 3: Fill Documentation

Edit the `.md` file using the template:
- Describe the changes
- List impact
- Provide verification steps
- Add testing procedures

### Step 4: Test Locally

```bash
# Apply migration
psql -U postgres -d auth_db_test -f migrations/YYYYMMDD_your_description.sql

# Verify it worked
psql -U postgres -d auth_db_test -c "\d table_name"

# Test rollback
psql -U postgres -d auth_db_test -f migrations/YYYYMMDD_your_description_rollback.sql

# Verify rollback worked
psql -U postgres -d auth_db_test -c "\d table_name"
```

### Step 5: Update Documentation

- [ ] Add to [MIGRATIONS_LOG.md](MIGRATIONS_LOG.md)
- [ ] Update [MIGRATIONS.md](../docs/migrations/MIGRATIONS.md) if needed
- [ ] Update [BREAKING_CHANGES.md](../docs/BREAKING_CHANGES.md) if breaking
- [ ] Update [UPGRADE_GUIDE.md](../docs/migrations/UPGRADE_GUIDE.md)
- [ ] Update [../CHANGELOG.md](../CHANGELOG.md)

### Step 6: Create PR

Create PR with:
- All migration files
- Updated documentation
- Label: "migration"
- Reviewers assigned

---

## Migration Types

### 1. Schema Migrations (SQL)

**Use for:**
- Adding/removing columns
- Changing column types
- Adding/removing tables
- Adding/removing indexes
- Adding/removing constraints

**Example:**
```sql
ALTER TABLE users ADD COLUMN preferences JSONB DEFAULT '{}';
CREATE INDEX idx_users_preferences ON users USING GIN (preferences);
```

### 2. Data Migrations (SQL)

**Use for:**
- Transforming existing data
- Populating new columns
- Cleaning up old data

**Example:**
```sql
UPDATE users 
SET preferences = '{"theme": "dark"}' 
WHERE preferences IS NULL;
```

### 3. GORM AutoMigrate

**Use for:**
- New tables from models
- New nullable columns
- Simple indexes

**How it works:**
- Add field to model in `pkg/models/`
- GORM creates it on next startup
- No SQL file needed

**Example:**
```go
type User struct {
    // ... existing fields
    NewField string `gorm:"type:varchar(100)" json:"new_field"`
}
```

---

## When to Use What

| Change Type | Use SQL | Use AutoMigrate |
|-------------|---------|-----------------|
| Add nullable column | Either | ✅ Preferred |
| Add NOT NULL column | ✅ Required | ❌ No |
| Change column type | ✅ Required | ❌ No |
| Remove column | ✅ Required | ❌ No |
| Rename column | ✅ Required | ❌ No |
| Add table | Either | ✅ Preferred |
| Add simple index | Either | ✅ Preferred |
| Add complex constraint | ✅ Required | ❌ No |
| Transform data | ✅ Required | ❌ No |
| Breaking change | ✅ Required | ❌ No |

---

## Best Practices

### 1. Always Use Transactions

```sql
BEGIN;
  -- Your changes
  ALTER TABLE users ADD COLUMN new_field VARCHAR(100);
COMMIT;
-- Add ROLLBACK if something fails
```

### 2. Make Migrations Idempotent

```sql
-- Use IF EXISTS / IF NOT EXISTS
ALTER TABLE users ADD COLUMN IF NOT EXISTS new_field VARCHAR(100);
CREATE INDEX IF NOT EXISTS idx_users_field ON users(new_field);
DROP COLUMN IF EXISTS old_field;
```

### 3. Test Thoroughly

- Test on copy of production data
- Test with realistic data volumes
- Test migration + rollback cycle
- Test application with migrated schema

### 4. Document Everything

Include in documentation:
- What changed and why
- Breaking changes
- Performance impact
- Rollback procedure
- Verification steps

### 5. Gradual Migrations for Breaking Changes

Instead of:
```sql
-- DON'T: Immediate breaking change
ALTER TABLE users DROP COLUMN old_email;
```

Do:
```sql
-- Step 1 (v1.1): Add new column
ALTER TABLE users ADD COLUMN new_email VARCHAR(255);

-- Step 2 (v1.1): Migrate data (in application code or script)
-- UPDATE users SET new_email = old_email;

-- Step 3 (v1.2): Make NOT NULL after verification
-- ALTER TABLE users ALTER COLUMN new_email SET NOT NULL;

-- Step 4 (v2.0): Drop old column
-- ALTER TABLE users DROP COLUMN old_email;
```

### 6. Consider Performance

```sql
-- For large tables, add index CONCURRENTLY
CREATE INDEX CONCURRENTLY idx_users_field ON users(field);

-- For large data migrations, do in batches
UPDATE users SET new_field = 'value' WHERE id IN (
    SELECT id FROM users WHERE new_field IS NULL LIMIT 1000
);
```

---

## Testing Migrations

### Local Testing

```bash
# 1. Create test database
createdb -U postgres auth_db_test

# 2. Restore production backup (optional)
psql -U postgres -d auth_db_test < prod_backup.sql

# 3. Test migration
psql -U postgres -d auth_db_test -f migrations/YYYYMMDD_migration.sql

# 4. Verify
psql -U postgres -d auth_db_test -c "\d table_name"

# 5. Test rollback
psql -U postgres -d auth_db_test -f migrations/YYYYMMDD_migration_rollback.sql

# 6. Verify rollback
psql -U postgres -d auth_db_test -c "\d table_name"

# 7. Cleanup
dropdb -U postgres auth_db_test
```

### Staging Testing

```bash
# Apply to staging first
export DB_NAME=auth_db_staging
./scripts/migrate.sh

# Monitor for issues
# Test application functionality
# Verify no errors in logs
```

---

## Rollback Procedure

### If Migration Fails

**1. Stop Application**
```bash
docker-compose down
```

**2. Check Error**
```bash
# Review PostgreSQL logs
docker logs auth_db

# Identify the issue
```

**3. Apply Rollback**
```bash
psql -U postgres -d auth_db -f migrations/YYYYMMDD_migration_rollback.sql
```

**4. Verify Rollback**
```bash
psql -U postgres -d auth_db -c "\d table_name"
```

**5. Fix Migration**
- Fix the SQL file
- Test locally
- Try again

### If Migration Succeeds But App Fails

**1. Rollback Application**
```bash
git checkout previous-version
docker-compose up -d
```

**2. Decide: Keep or Rollback Migration**

Option A: Keep migration, fix app
```bash
# Fix application code
# Deploy fixed version
```

Option B: Rollback everything
```bash
# Stop app
docker-compose down

# Rollback migration
psql -U postgres -d auth_db -f migrations/YYYYMMDD_migration_rollback.sql

# Start old version
git checkout previous-version
docker-compose up -d
```

---

## Migration Checklist

Before creating migration:
- [ ] Understand the requirement
- [ ] Design the schema changes
- [ ] Consider backward compatibility
- [ ] Plan rollback strategy

Creating migration:
- [ ] Use template for documentation
- [ ] Create forward migration SQL
- [ ] Create rollback migration SQL
- [ ] Test locally (apply + rollback)
- [ ] Test with realistic data
- [ ] Update MIGRATIONS_LOG.md
- [ ] Update other docs if breaking

Before merging:
- [ ] Code review completed
- [ ] Tested in development
- [ ] Tested in staging
- [ ] Documentation reviewed
- [ ] Approved by maintainer

Before production:
- [ ] Backup created
- [ ] Maintenance window scheduled (if needed)
- [ ] Rollback plan ready
- [ ] Team notified

---

## Common Issues

### "relation already exists"

**Cause:** Migration already applied

**Solution:**
```sql
-- Use IF NOT EXISTS
CREATE TABLE IF NOT EXISTS table_name (...);
ALTER TABLE table_name ADD COLUMN IF NOT EXISTS column_name TYPE;
```

### "column does not exist"

**Cause:** Trying to modify non-existent column

**Solution:**
```sql
-- Check if column exists first
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.columns 
               WHERE table_name='table' AND column_name='column') THEN
        ALTER TABLE table ALTER COLUMN column ...;
    END IF;
END $$;
```

### "constraint violation"

**Cause:** Existing data violates new constraint

**Solution:**
```sql
-- Clean data first
UPDATE table SET column = default_value WHERE condition;

-- Then add constraint
ALTER TABLE table ADD CONSTRAINT ...;
```

---

## See Also

- [MIGRATIONS.md](../docs/migrations/MIGRATIONS.md) - User migration guide
- [BREAKING_CHANGES.md](../docs/BREAKING_CHANGES.md) - Breaking changes tracker
- [UPGRADE_GUIDE.md](../docs/migrations/UPGRADE_GUIDE.md) - Version upgrade guide
- [TEMPLATE.md](TEMPLATE.md) - Migration template
- [MIGRATIONS_LOG.md](MIGRATIONS_LOG.md) - Applied migrations log
- [../CONTRIBUTING.md](../CONTRIBUTING.md) - Contribution guidelines

---

## Support

Need help with migrations?

- 📖 Read the docs above
- 🔍 Check existing migrations for examples
- 💬 Ask in GitHub Discussions
- 🐛 Open an issue
- 📧 Contact maintainers

---

*Last Updated: 2024-01-03*

