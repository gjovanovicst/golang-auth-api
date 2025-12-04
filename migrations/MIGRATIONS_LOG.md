# Migrations Log

This file tracks all applied database migrations in chronological order.

---

## Purpose

- **Historical record** of all database changes
- **Version tracking** for deployments
- **Dependencies** between migrations
- **Audit trail** for schema evolution

---

## Current Database Version

**Latest Migration:** `20240103_add_activity_log_smart_fields`  
**Database Schema Version:** v1.1.0  
**Compatible Application Version:** v1.1.0+

---

## Applied Migrations

### 2024-01-03: Smart Activity Logging System

| Field | Value |
|-------|-------|
| **Migration ID** | `20240103_add_activity_log_smart_fields` |
| **Date Applied** | 2024-01-03 |
| **App Version** | v1.1.0 |
| **Type** | Schema + Data |
| **Breaking** | No |
| **Status** | ✅ Applied |

**Description:**
Implements smart activity logging with severity classification, automatic expiration, and anomaly detection. Reduces database size by 80-95% while maintaining security audit capabilities.

**Files:**
- `migrations/20240103_add_activity_log_smart_fields.sql`
- `migrations/20240103_add_activity_log_smart_fields_rollback.sql`
- `migrations/20240103_add_activity_log_smart_fields.md`

**Changes:**
- Added `severity` column (CRITICAL, IMPORTANT, INFORMATIONAL)
- Added `expires_at` column for automatic cleanup
- Added `is_anomaly` column for anomaly detection
- Created 3 new indexes for performance
- Updated all existing logs with appropriate values

**Impact:**
- Database size reduction: 80-95% (after cleanup)
- Query performance: Improved
- Downtime required: None
- Rollback available: Yes

**Dependencies:**
- None (initial smart logging implementation)

**Documentation:**
- [Migration Details](20240103_add_activity_log_smart_fields.md)
- [Activity Logging Guide](../docs/ACTIVITY_LOGGING_GUIDE.md)
- [Upgrade Guide](../UPGRADE_GUIDE.md)

---

### Initial Schema (v1.0.0)

| Field | Value |
|-------|-------|
| **Migration ID** | `initial_schema` |
| **Date Applied** | 2024-01-01 (estimated) |
| **App Version** | v1.0.0 |
| **Type** | Initial Schema |
| **Breaking** | N/A |
| **Status** | ✅ Applied |

**Description:**
Initial database schema for Authentication API.

**Tables Created:**
- `users` - User accounts
- `activity_logs` - User activity tracking (basic)
- `two_factor_auth` - 2FA settings
- Other authentication-related tables

**Changes:**
- Complete initial schema via GORM AutoMigrate
- All core authentication tables
- Basic indexes and constraints

**Impact:**
- Initial database setup
- No migration required (fresh install)

**Dependencies:**
- None (initial release)

---

## Migration History Summary

| # | Date | Migration ID | Version | Type | Breaking | Status |
|---|------|--------------|---------|------|----------|--------|
| 2 | 2024-01-03 | `20240103_add_activity_log_smart_fields` | v1.1.0 | Schema + Data | No | ✅ |
| 1 | 2024-01-01 | `initial_schema` | v1.0.0 | Initial | No | ✅ |

---

## Migration Dependencies

```
initial_schema (v1.0.0)
    └── 20240103_add_activity_log_smart_fields (v1.1.0)
            └── [Future migrations will be added here]
```

---

## Version Compatibility

| App Version | Database Schema | Min DB Version | Max DB Version | Notes |
|-------------|-----------------|----------------|----------------|-------|
| v1.0.0 | v1.0.0 | v1.0.0 | v1.0.0 | Initial release |
| v1.1.0 | v1.1.0 | v1.0.0 | v1.1.0 | Backward compatible with v1.0.0 |

---

## Rollback History

No rollbacks have been performed.

**Format for rollback entries:**
```
Date: YYYY-MM-DD
Migration: migration_id
Reason: Why rollback was needed
Result: Success/Failed
Notes: Any additional information
```

---

## Pending Migrations

No pending migrations at this time.

**Future migrations planned:**
- Role-based access control (RBAC) - v1.2.0 (planned)
- User preferences storage - v1.3.0 (planned)

---

## How to Update This Log

When creating a new migration:

1. **Add entry to "Applied Migrations" section** with all details
2. **Update "Current Database Version"** at top
3. **Add to "Migration History Summary" table**
4. **Update "Migration Dependencies" tree**
5. **Update "Version Compatibility" table**
6. **Move from "Pending" to "Applied"** if applicable

### Template for New Entry

```markdown
### YYYY-MM-DD: [Migration Title]

| Field | Value |
|-------|-------|
| **Migration ID** | `YYYYMMDD_migration_name` |
| **Date Applied** | YYYY-MM-DD |
| **App Version** | vX.X.X |
| **Type** | Schema / Data / Breaking |
| **Breaking** | Yes / No |
| **Status** | ✅ Applied / ⏳ Pending / ❌ Failed |

**Description:**
Brief description of what this migration does.

**Files:**
- `migrations/YYYYMMDD_migration_name.sql`
- `migrations/YYYYMMDD_migration_name_rollback.sql`
- `migrations/YYYYMMDD_migration_name.md`

**Changes:**
- Change 1
- Change 2
- Change 3

**Impact:**
- Performance impact
- Downtime requirements
- Rollback availability

**Dependencies:**
- Depends on: [previous_migration_id]
- Required for: [future_migration_id]

**Documentation:**
- [Migration Details](YYYYMMDD_migration_name.md)
- [Relevant Guide](../docs/guide.md)
```

---

## Statistics

**Total Migrations:** 2  
**Successful Migrations:** 2 (100%)  
**Failed Migrations:** 0  
**Rollbacks Performed:** 0  
**Breaking Changes:** 0  

**By Type:**
- Schema Changes: 1
- Data Migrations: 1
- Initial Schema: 1

**By Version:**
- v1.0.0: 1 migration
- v1.1.0: 1 migration

---

## Maintenance

### Review Schedule

This log should be reviewed:
- ✅ Before each release
- ✅ After each migration
- ✅ During version planning
- ✅ When troubleshooting issues

### Cleanup Policy

- Keep all migration entries (never delete)
- Archive very old entries (after 2+ years)
- Maintain complete history for audit

---

## See Also

- [MIGRATIONS.md](../MIGRATIONS.md) - User migration guide
- [BREAKING_CHANGES.md](../BREAKING_CHANGES.md) - Breaking changes tracker
- [UPGRADE_GUIDE.md](../UPGRADE_GUIDE.md) - Version upgrade instructions
- [migrations/README.md](README.md) - Developer migration guide
- [migrations/TEMPLATE.md](TEMPLATE.md) - Migration template

---

## Notes

- All timestamps in UTC
- Migration IDs must be unique
- Always document breaking changes thoroughly
- Test rollback procedures before production deployment
- Keep documentation up to date

---

*Last Updated: 2024-01-03*  
*Maintained by: Project Maintainers*  
*Review Frequency: Per Release*

