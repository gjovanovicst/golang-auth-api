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

**Latest Migration:** `20260314_add_app_link_paths`  
**Database Schema Version:** v1.4.0  
**Compatible Application Version:** v1.4.0+

---

## Applied Migrations

### 2026-03-14: Configurable Email Action Link Paths

| Field | Value |
|-------|-------|
| **Migration ID** | `20260314_add_app_link_paths` |
| **Date Applied** | 2026-03-14 |
| **App Version** | v1.4.0 |
| **Type** | Schema Change |
| **Breaking** | No |
| **Status** | ✅ Applied |

**Description:**
Adds per-application configurable URL path suffixes for email action links
(password-reset, magic-link, and email-verification). When a field is left
empty the system falls back to its original hardcoded default path, so all
existing integrations continue to work without change.

**Files:**
- `migrations/20260314_add_app_link_paths.sql`
- `migrations/20260314_add_app_link_paths_rollback.sql`

**Changes:**
- Extended `applications` with 3 new columns: `reset_password_path`, `magic_link_path`, `verify_email_path` (all `VARCHAR(500) NOT NULL DEFAULT ''`)
- Shared URL-resolution utility extracted to `internal/util/frontend_url.go`
- Duplicate `resolveAppFrontendURL` function removed from `internal/user/service.go` and `internal/email/resolver.go`

**Impact:**
- Database size: negligible (3 short VARCHAR columns)
- Query performance: no regression
- Downtime required: None
- Rollback available: Yes (drops the 3 columns)

**Dependencies:**
- `20260105_add_multi_tenancy` (requires `applications` table)

---

### 2026-03-06: OIDC Provider Support

| Field | Value |
|-------|-------|
| **Migration ID** | `20260306_add_oidc` |
| **Date Applied** | 2026-03-06 |
| **App Version** | v1.3.0 |
| **Type** | Schema Change |
| **Breaking** | No |
| **Status** | ✅ Applied |

**Description:**
Adds the OIDC Provider feature (feature #12). Each Application can now act as
a standards-compliant OIDC issuer (RS256 ID tokens, Authorization Code + PKCE,
Client Credentials). Relying-party clients are registered per application.

**Files:**
- `migrations/20260306_add_oidc.sql`
- `migrations/20260306_add_oidc_rollback.sql`
- `migrations/20260306_add_oidc.md`

**Changes:**
- Extended `applications` with 4 new OIDC columns: `oidc_enabled`, `oidc_rsa_private_key`, `oidc_id_token_ttl`, `oidc_issuer_url`
- Created `oidc_clients` table (relying-party client registry, FK → `applications` ON DELETE CASCADE)
- Created `oidc_auth_codes` table (single-use authorization codes with expiry + replay protection)
- 6 supporting indexes across both new tables

**Impact:**
- Database size: negligible until OIDC clients are created
- Query performance: no regression on existing tables
- Downtime required: None
- Rollback available: Yes (destructive — drops both tables and 4 columns)

**Dependencies:**
- `20260105_add_multi_tenancy` (requires `applications` table)

**Documentation:**
- [Migration Details](20260306_add_oidc.md)

---

### 2026-03-05: Webhook System

| Field | Value |
|-------|-------|
| **Migration ID** | `20260305_add_webhooks` |
| **Date Applied** | 2026-03-05 |
| **App Version** | v1.2.0 |
| **Type** | Schema Change |
| **Breaking** | No |
| **Status** | ✅ Applied |

**Description:**
Adds the webhook system (feature #11). Applications can register HTTP endpoint
URLs that receive HMAC-SHA256-signed POST payloads when auth events fire.
Delivery attempts are fully logged with retry scheduling.

**Files:**
- `migrations/20260305_add_webhooks.sql`
- `migrations/20260305_add_webhooks_rollback.sql`
- `migrations/20260305_add_webhooks.md`

**Changes:**
- Created `webhook_endpoints` table (one URL per app/event-type pair, soft-delete)
- Created `webhook_deliveries` table (full delivery history + retry tracking)
- Composite partial unique index on `(app_id, event_type) WHERE deleted_at IS NULL`
- FK → `applications(id)` ON DELETE CASCADE on `webhook_endpoints`
- FK → `webhook_endpoints(id)` ON DELETE CASCADE on `webhook_deliveries`
- CHECK constraint enforcing 8 valid event types
- 7 supporting indexes for delivery history and retry-worker queries

**Impact:**
- Database size: negligible until webhooks are actively used
- Query performance: no regression on existing tables
- Downtime required: None
- Rollback available: Yes

**Dependencies:**
- `20260105_add_multi_tenancy` (requires `applications` table)

**Documentation:**
- [Migration Details](20260305_add_webhooks.md)

---

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
| 4 | 2026-03-06 | `20260306_add_oidc` | v1.3.0 | Schema Change | No | ✅ |
| 3 | 2026-03-05 | `20260305_add_webhooks` | v1.2.0 | Schema Change | No | ✅ |
| 2 | 2024-01-03 | `20240103_add_activity_log_smart_fields` | v1.1.0 | Schema + Data | No | ✅ |
| 1 | 2024-01-01 | `initial_schema` | v1.0.0 | Initial | No | ✅ |

---

## Migration Dependencies

```
initial_schema (v1.0.0)
    └── 20240103_add_activity_log_smart_fields (v1.1.0)
            └── 20260305_add_webhooks (v1.2.0)
                    └── 20260306_add_oidc (v1.3.0)
                                └── [Future migrations will be added here]
```

---

## Version Compatibility

| App Version | Database Schema | Min DB Version | Max DB Version | Notes |
|-------------|-----------------|----------------|----------------|-------|
| v1.0.0 | v1.0.0 | v1.0.0 | v1.0.0 | Initial release |
| v1.1.0 | v1.1.0 | v1.0.0 | v1.1.0 | Backward compatible with v1.0.0 |
| v1.2.0 | v1.2.0 | v1.1.0 | v1.2.0 | Backward compatible with v1.1.0 |
| v1.3.0 | v1.3.0 | v1.2.0 | v1.3.0 | Backward compatible with v1.2.0 |

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

**Total Migrations:** 4  
**Successful Migrations:** 4 (100%)  
**Failed Migrations:** 0  
**Rollbacks Performed:** 0  
**Breaking Changes:** 0  

**By Type:**
- Schema Changes: 3
- Data Migrations: 1
- Initial Schema: 1

**By Version:**
- v1.0.0: 1 migration
- v1.1.0: 1 migration
- v1.2.0: 1 migration
- v1.3.0: 1 migration

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

*Last Updated: 2026-03-06*  
*Maintained by: Project Maintainers*  
*Review Frequency: Per Release*

