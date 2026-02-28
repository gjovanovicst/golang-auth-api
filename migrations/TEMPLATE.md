# Migration Template

Use this template when creating new database migrations.

---

## Migration: [Short Description]

**Date:** YYYY-MM-DD  
**Version:** vX.X.X  
**Type:** Schema Change / Data Migration / Breaking Change  
**Breaking:** Yes / No

---

## Overview

Brief description of what this migration does and why it's needed.

**Example:**
> This migration adds support for user preferences by adding a preferences column to the users table.

---

## Changes

### Database Schema

List all schema changes:

**Tables Modified:**
- `table_name` - description of changes

**Columns Added:**
- `table_name.column_name` - TYPE - description

**Columns Modified:**
- `table_name.column_name` - old TYPE → new TYPE - reason

**Columns Removed:**
- `table_name.column_name` - reason for removal

**Indexes Added:**
- `index_name` on `table_name(columns)` - reason

**Constraints Added:**
- Description of constraints

### Data Changes

If this migration includes data transformations:

**Data Modified:**
- Description of what data is changed
- Why it's being changed
- How existing data is migrated

---

## Migration Files

**Forward Migration:**
```
migrations/YYYYMMDD_HHMMSS_description.sql
```

**Rollback Migration:**
```
migrations/YYYYMMDD_HHMMSS_description_rollback.sql
```

---

## Impact Assessment

### Breaking Changes

**Is this breaking?** Yes / No

If yes, describe what breaks:
- API endpoints affected
- Client code changes needed
- Configuration changes needed

### Performance Impact

- Expected migration time: X seconds/minutes
- Impact on application during migration: None / Downtime required
- Database size impact: +/- X MB

### Compatibility

- **Backward Compatible:** Yes / No
- **Forward Compatible:** Yes / No
- **Minimum App Version:** vX.X.X
- **Requires Configuration Changes:** Yes / No

---

## Migration Steps

### Prerequisites

- [ ] Application version X.X.X or higher
- [ ] Database backup completed
- [ ] Tested in development environment
- [ ] Tested in staging environment

### Applying Migration

**Development:**
```bash
# Automatic via GORM
make docker-dev

# OR Manual
psql -U postgres -d auth_db -f migrations/YYYYMMDD_description.sql
```

**Production:**
```bash
# 1. Backup
pg_dump -U postgres -d auth_db > backup_$(date +%Y%m%d_%H%M%S).sql

# 2. Stop application (if needed)
docker-compose down

# 3. Apply migration
psql -U postgres -d auth_db -f migrations/YYYYMMDD_description.sql

# 4. Verify
psql -U postgres -d auth_db -c "\d table_name"

# 5. Start application
docker-compose up -d
```

### Rollback Steps

If migration fails or needs to be rolled back:

```bash
# 1. Stop application
docker-compose down

# 2. Apply rollback
psql -U postgres -d auth_db -f migrations/YYYYMMDD_description_rollback.sql

# 3. Verify rollback
psql -U postgres -d auth_db -c "\d table_name"

# 4. Restore from backup (if needed)
psql -U postgres -d auth_db < backup_YYYYMMDD_HHMMSS.sql

# 5. Checkout previous version
git checkout vX.X.X

# 6. Start application
docker-compose up -d
```

---

## Verification

### Post-Migration Checks

**Database Verification:**
```sql
-- Check new columns exist
\d table_name

-- Check data integrity
SELECT COUNT(*) FROM table_name WHERE new_column IS NOT NULL;

-- Check indexes
\di table_name*

-- Check constraints
SELECT * FROM pg_constraint WHERE conname = 'constraint_name';
```

**Application Verification:**
- [ ] Application starts successfully
- [ ] No errors in logs
- [ ] API endpoints respond correctly
- [ ] New features work as expected
- [ ] Existing features still work

---

## Code Changes Required

### API Changes

**New Endpoints:**
- `GET /path` - description

**Modified Endpoints:**
- `PUT /path` - what changed

**Removed Endpoints:**
- `DELETE /path` - migration path

### Model Changes

**Go Models:**
```go
// Add to pkg/models/model_name.go
type Model struct {
    // ... existing fields
    NewField string `gorm:"type:varchar(100)" json:"new_field"`
}
```

### Configuration Changes

**Environment Variables:**
```bash
# Add to .env
NEW_CONFIG_OPTION=value
```

**Defaults:**
- If not specified, defaults to: `default_value`

---

## Testing

### Test Cases

**Unit Tests:**
- [ ] Test new model fields
- [ ] Test validation logic
- [ ] Test repository methods

**Integration Tests:**
- [ ] Test migration applies cleanly
- [ ] Test rollback works
- [ ] Test API endpoints
- [ ] Test with existing data

**Manual Testing:**
```bash
# 1. Apply migration in test environment
# 2. Create test data
# 3. Verify new functionality
# 4. Test rollback
# 5. Verify data integrity
```

---

## Documentation Updates

Files to update:

- [ ] [MIGRATIONS.md](../docs/migrations/MIGRATIONS.md) - Add to current migrations list
- [ ] [BREAKING_CHANGES.md](../docs/BREAKING_CHANGES.md) - If breaking, add entry
- [ ] [UPGRADE_GUIDE.md](../docs/migrations/UPGRADE_GUIDE.md) - Add upgrade instructions
- [ ] [CHANGELOG.md](../CHANGELOG.md) - Add to version changelog
- [ ] [migrations/MIGRATIONS_LOG.md](MIGRATIONS_LOG.md) - Add to log
- [ ] [README.md](../README.md) - Update if features changed
- [ ] [docs/API.md](../docs/API.md) - Update API documentation
- [ ] Swagger annotations - Update if API changed

---

## Example SQL

### Forward Migration (migrations/YYYYMMDD_description.sql)

```sql
-- Migration: Short Description
-- Date: YYYY-MM-DD
-- Version: vX.X.X

BEGIN;

-- Add new column
ALTER TABLE table_name 
ADD COLUMN IF NOT EXISTS new_column VARCHAR(100);

-- Add index
CREATE INDEX IF NOT EXISTS idx_table_column 
ON table_name(new_column);

-- Add constraint
ALTER TABLE table_name 
ADD CONSTRAINT chk_table_column 
CHECK (new_column IS NOT NULL);

-- Update existing data (if needed)
UPDATE table_name 
SET new_column = 'default_value' 
WHERE new_column IS NULL;

-- Add comment for documentation
COMMENT ON COLUMN table_name.new_column IS 'Description of column purpose';

COMMIT;
```

### Rollback Migration (migrations/YYYYMMDD_description_rollback.sql)

```sql
-- Rollback: Short Description
-- Date: YYYY-MM-DD
-- Version: vX.X.X

BEGIN;

-- Remove constraint
ALTER TABLE table_name 
DROP CONSTRAINT IF EXISTS chk_table_column;

-- Remove index
DROP INDEX IF EXISTS idx_table_column;

-- Remove column
ALTER TABLE table_name 
DROP COLUMN IF EXISTS new_column;

COMMIT;
```

---

## Checklist

Before submitting PR:

- [ ] Migration SQL files created (up + down)
- [ ] Migration documentation completed (this file)
- [ ] Tested locally (apply + rollback)
- [ ] Tested with realistic data volume
- [ ] MIGRATIONS.md updated
- [ ] BREAKING_CHANGES.md updated (if breaking)
- [ ] UPGRADE_GUIDE.md updated
- [ ] CHANGELOG.md updated
- [ ] MIGRATIONS_LOG.md updated
- [ ] Code changes implemented
- [ ] Tests added/updated
- [ ] Documentation updated
- [ ] PR created with "migration" label
- [ ] Reviewers assigned

---

## Notes

Any additional notes, caveats, or special instructions:

- Note 1
- Note 2
- Note 3

---

## References

- Related Issue: #XXX
- Related PR: #XXX
- Documentation: [link]
- Discussion: [link]

---

*Template Version: 1.0*  
*Last Updated: 2024-01-03*

