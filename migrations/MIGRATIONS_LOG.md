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
**Database Schema Version:** v1.0.0-alpha.6  
**Compatible Application Version:** v1.0.0-alpha.6+

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

### 2026-03-11: Application Customization (Branding, Password Policy, Token TTLs)

| Field | Value |
|-------|-------|
| **Migration ID** | `20260311_add_app_customization` |
| **Date Applied** | 2026-03-11 |
| **App Version** | v1.0.0-alpha.6 |
| **Type** | Schema Change |
| **Breaking** | No |
| **Status** | ✅ Applied |

**Description:**
Adds per-application login page branding, password policy enforcement, and access/refresh token TTL overrides to the `applications` table. Also adds `password_history` (JSONB) and `password_changed_at` columns to `users` to support password reuse prevention and age enforcement.

**Files:**
- `migrations/20260311_add_app_customization.sql`

**Changes:**
- Added 4 branding columns to `applications`: `login_logo_url`, `login_primary_color`, `login_secondary_color`, `login_display_name`
- Added 8 password policy columns to `applications`: `pw_min_length`, `pw_max_length`, `pw_require_upper`, `pw_require_lower`, `pw_require_digit`, `pw_require_symbol`, `pw_history_count`, `pw_max_age_days`
- Added 2 token TTL override columns to `applications`: `access_token_ttl_minutes`, `refresh_token_ttl_hours` (0 = use global env var defaults)
- Added `password_history` (JSONB) and `password_changed_at` (TIMESTAMPTZ) to `users`

**Impact:**
- Database size: negligible (fixed-width columns, sparse JSONB)
- Query performance: no regression
- Downtime required: None
- Rollback available: No dedicated rollback file (use `ALTER TABLE … DROP COLUMN`)

**Dependencies:**
- `20260105_add_multi_tenancy` (requires `applications` and `users` tables)

---

### 2026-03-10: 2FA Previous Method Tracking

| Field | Value |
|-------|-------|
| **Migration ID** | `20260310_add_two_fa_previous_method` |
| **Date Applied** | 2026-03-10 |
| **App Version** | v1.0.0-alpha.6 |
| **Type** | Schema Change |
| **Breaking** | No |
| **Status** | ✅ Applied |

**Description:**
Adds `two_fa_previous_method` and `two_fa_previous_secret` columns to `users`. These preserve the original 2FA method and secret when a user temporarily switches to backup-email 2FA, allowing automatic restoration of the original method when backup-email 2FA is disabled.

**Files:**
- `migrations/20260310_add_two_fa_previous_method.sql`

**Changes:**
- Added `two_fa_previous_method VARCHAR(20) NOT NULL DEFAULT ''` to `users`
- Added `two_fa_previous_secret TEXT NOT NULL DEFAULT ''` to `users`

**Impact:**
- Database size: negligible
- Query performance: no regression
- Downtime required: None
- Rollback available: No dedicated rollback file (drop columns)

**Dependencies:**
- `initial_schema` (requires `users` table)

---

### 2026-03-09: Backup Email Verification Email Type

| Field | Value |
|-------|-------|
| **Migration ID** | `20260309_seed_backup_email_verification_type` |
| **Date Applied** | 2026-03-09 |
| **App Version** | v1.0.0-alpha.6 |
| **Type** | Data |
| **Breaking** | No |
| **Status** | ✅ Applied |

**Description:**
Seeds the `backup_email_verification` email type and its global default template into the email system. This type was already implemented in Go code (`internal/email/defaults.go`) but was absent from the database, preventing admin customization via the GUI.

**Files:**
- `migrations/20260309_seed_backup_email_verification_type.sql`

**Changes:**
- Inserted `backup_email_verification` row into `email_types`
- Inserted global default Go-template into `email_templates` (subject: "Verify Your Backup Email Address"; variables: `app_name`, `backup_email`, `verification_link`, `expiration_minutes`)

**Impact:**
- Database size: negligible (2 rows)
- Query performance: no regression
- Downtime required: None
- Rollback available: No (DELETE the seeded rows manually if needed)

**Dependencies:**
- `initial_schema` (requires `email_types` and `email_templates` tables)

---

### 2026-03-05: Security Alert Email Types

| Field | Value |
|-------|-------|
| **Migration ID** | `20260305_seed_security_email_types` |
| **Date Applied** | 2026-03-05 |
| **App Version** | v1.0.0-alpha.6 |
| **Type** | Data |
| **Breaking** | No |
| **Status** | ✅ Applied |

**Description:**
Seeds the `new_device_login` and `suspicious_activity` email types with global default templates. These types were already used in Go code but were missing from the database.

**Files:**
- `migrations/20260305_seed_security_email_types.sql`

**Changes:**
- Inserted `new_device_login` row into `email_types` (variables: `app_name`, `user_email`, `login_ip`, `login_location`, `login_device`, `login_time`)
- Inserted `suspicious_activity` row into `email_types` (adds `alert_type`, `alert_details` variables)
- Inserted 2 global default Go-templates into `email_templates`

**Impact:**
- Database size: negligible (4 rows)
- Query performance: no regression
- Downtime required: None
- Rollback available: No (DELETE the seeded rows manually if needed)

**Dependencies:**
- `initial_schema` (requires `email_types` and `email_templates` tables)

---

### 2026-03-05: API Key Expiring Soon Email Type

| Field | Value |
|-------|-------|
| **Migration ID** | `20260305_seed_api_key_expiring_soon_email_type` |
| **Date Applied** | 2026-03-05 |
| **App Version** | v1.0.0-alpha.6 |
| **Type** | Data |
| **Breaking** | No |
| **Status** | ✅ Applied |

**Description:**
Seeds the `api_key_expiring_soon` email type and its global default template. Used by the background API key expiry notification service to send 7-day and 1-day warnings to admins before keys expire.

**Files:**
- `migrations/20260305_seed_api_key_expiring_soon_email_type.sql`

**Changes:**
- Inserted `api_key_expiring_soon` row into `email_types` (variables: `app_name`, `api_key_name`, `api_key_prefix`, `api_key_type`, `api_key_expires_at`, `days_until_expiry`)
- Inserted global default Go-template into `email_templates`

**Impact:**
- Database size: negligible (2 rows)
- Query performance: no regression
- Downtime required: None
- Rollback available: No (DELETE the seeded rows manually if needed)

**Dependencies:**
- `initial_schema` (requires `email_types` and `email_templates` tables)
- `20260105_add_multi_tenancy` (requires `api_keys` table context)

---

### 2026-03-05: API Key Usage Analytics Table

| Field | Value |
|-------|-------|
| **Migration ID** | `20260305_create_api_key_usages` |
| **Date Applied** | 2026-03-05 |
| **App Version** | v1.0.0-alpha.6 |
| **Type** | Schema Change |
| **Breaking** | No |
| **Status** | ✅ Applied |

**Description:**
Creates the `api_key_usages` table for per-key daily request counters. Rows are upserted from middleware via fire-and-forget increments using `ON CONFLICT DO UPDATE`.

**Files:**
- `migrations/20260305_create_api_key_usages.sql`

**Changes:**
- Created `api_key_usages` table (`id BIGSERIAL`, `api_key_id UUID FK → api_keys ON DELETE CASCADE`, `period_date DATE`, `request_count BIGINT`, `updated_at TIMESTAMPTZ`)
- Composite unique index `idx_api_key_usage_key_period` on `(api_key_id, period_date)` — enables upsert
- Supporting index `idx_api_key_usages_api_key_id` on `api_key_id`

**Impact:**
- Database size: grows with API key usage volume
- Query performance: no regression on existing tables; upsert is O(1) via unique index
- Downtime required: None
- Rollback available: No dedicated rollback file (`DROP TABLE api_key_usages`)

**Dependencies:**
- `20260105_add_multi_tenancy` (requires `api_keys` table)

---

### 2026-03-05: Per-Application Brute-Force Protection Settings

| Field | Value |
|-------|-------|
| **Migration ID** | `20260305_add_app_bruteforce_settings` |
| **Date Applied** | 2026-03-05 |
| **App Version** | v1.0.0-alpha.6 |
| **Type** | Schema Change |
| **Breaking** | No |
| **Status** | ✅ Applied |

**Description:**
Adds nullable brute-force protection configuration columns to the `applications` table. `NULL` means "use the global default from environment variables"; non-NULL values override the global defaults for that specific application. Covers account lockout, progressive delays, and CAPTCHA triggers.

**Files:**
- `migrations/20260305_add_app_bruteforce_settings.sql`

**Changes:**
- Added 5 lockout columns: `bf_lockout_enabled`, `bf_lockout_threshold`, `bf_lockout_durations`, `bf_lockout_window`, `bf_lockout_tier_ttl`
- Added 4 progressive-delay columns: `bf_delay_enabled`, `bf_delay_start_after`, `bf_delay_max_seconds`, `bf_delay_tier_ttl`
- Added 4 CAPTCHA columns: `bf_captcha_enabled`, `bf_captcha_site_key`, `bf_captcha_secret_key`, `bf_captcha_threshold`

**Impact:**
- Database size: negligible (sparse nullable columns)
- Query performance: no regression
- Downtime required: None
- Rollback available: No dedicated rollback file (drop the 13 columns)

**Dependencies:**
- `20260105_add_multi_tenancy` (requires `applications` table)

---

### 2026-03-05: API Key Scopes and Expiry Notification Columns

| Field | Value |
|-------|-------|
| **Migration ID** | `20260305_add_api_key_scopes_columns` |
| **Date Applied** | 2026-03-05 |
| **App Version** | v1.0.0-alpha.6 |
| **Type** | Schema Change |
| **Breaking** | No |
| **Status** | ✅ Applied |

**Description:**
Extends the `api_keys` table with a `scopes` column for granular permission strings and two notification-tracking timestamps used by the expiry warning service to prevent duplicate emails.

**Files:**
- `migrations/20260305_add_api_key_scopes_columns.sql`

**Changes:**
- Added `scopes TEXT NOT NULL DEFAULT ''` to `api_keys` (comma-separated `resource:action` strings, e.g. `users:read,auth:*`)
- Added `notified_7_days_at TIMESTAMPTZ` to `api_keys`
- Added `notified_1_day_at TIMESTAMPTZ` to `api_keys`

**Impact:**
- Database size: negligible
- Query performance: no regression
- Downtime required: None
- Rollback available: No dedicated rollback file (drop the 3 columns)

**Dependencies:**
- `20260105_add_multi_tenancy` (requires `api_keys` table)

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
| 13 | 2026-03-14 | `20260314_add_app_link_paths` | v1.0.0-alpha.6 | Schema Change | No | ✅ |
| 12 | 2026-03-11 | `20260311_add_app_customization` | v1.0.0-alpha.6 | Schema Change | No | ✅ |
| 11 | 2026-03-10 | `20260310_add_two_fa_previous_method` | v1.0.0-alpha.6 | Schema Change | No | ✅ |
| 10 | 2026-03-09 | `20260309_seed_backup_email_verification_type` | v1.0.0-alpha.6 | Data | No | ✅ |
| 9 | 2026-03-06 | `20260306_add_oidc` | v1.0.0-alpha.6 | Schema Change | No | ✅ |
| 8 | 2026-03-05 | `20260305_seed_security_email_types` | v1.0.0-alpha.6 | Data | No | ✅ |
| 7 | 2026-03-05 | `20260305_seed_api_key_expiring_soon_email_type` | v1.0.0-alpha.6 | Data | No | ✅ |
| 6 | 2026-03-05 | `20260305_create_api_key_usages` | v1.0.0-alpha.6 | Schema Change | No | ✅ |
| 5 | 2026-03-05 | `20260305_add_app_bruteforce_settings` | v1.0.0-alpha.6 | Schema Change | No | ✅ |
| 4 | 2026-03-05 | `20260305_add_api_key_scopes_columns` | v1.0.0-alpha.6 | Schema Change | No | ✅ |
| 3 | 2026-03-05 | `20260305_add_webhooks` | v1.0.0-alpha.6 | Schema Change | No | ✅ |
| 2 | 2024-01-03 | `20240103_add_activity_log_smart_fields` | v1.1.0 | Schema + Data | No | ✅ |
| 1 | 2024-01-01 | `initial_schema` | v1.0.0 | Initial | No | ✅ |

---

## Migration Dependencies

```
initial_schema (v1.0.0)
    ├── 20240103_add_activity_log_smart_fields (v1.1.0)
    └── [multi-tenancy prerequisite for all 2026 migrations]
            ├── 20260305_add_api_key_scopes_columns (v1.0.0-alpha.6)
            │       └── 20260305_seed_api_key_expiring_soon_email_type (v1.0.0-alpha.6)
            ├── 20260305_add_app_bruteforce_settings (v1.0.0-alpha.6)
            ├── 20260305_create_api_key_usages (v1.0.0-alpha.6)
            ├── 20260305_seed_security_email_types (v1.0.0-alpha.6)
            ├── 20260305_add_webhooks (v1.0.0-alpha.6)
            ├── 20260306_add_oidc (v1.0.0-alpha.6)
            ├── 20260309_seed_backup_email_verification_type (v1.0.0-alpha.6)
            ├── 20260310_add_two_fa_previous_method (v1.0.0-alpha.6)
            ├── 20260311_add_app_customization (v1.0.0-alpha.6)
            └── 20260314_add_app_link_paths (v1.0.0-alpha.6)
```

---

## Version Compatibility

| App Version | Database Schema | Min DB Version | Max DB Version | Notes |
|-------------|-----------------|----------------|----------------|-------|
| v1.0.0 | v1.0.0 | v1.0.0 | v1.0.0 | Initial release |
| v1.1.0 | v1.1.0 | v1.0.0 | v1.1.0 | Backward compatible with v1.0.0 |
| v1.2.0 | v1.2.0 | v1.1.0 | v1.2.0 | Backward compatible with v1.1.0 |
| v1.3.0 | v1.3.0 | v1.2.0 | v1.3.0 | Backward compatible with v1.2.0 |
| v1.0.0-alpha.6 | v1.0.0-alpha.6 | v1.3.0 | v1.0.0-alpha.6 | Adds OIDC, Webhooks, Brute-Force, GeoIP, API Key Scopes/Usage, App Customization, Backup Email 2FA, Trusted Devices |

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

**Total Migrations:** 13  
**Successful Migrations:** 13 (100%)  
**Failed Migrations:** 0  
**Rollbacks Performed:** 0  
**Breaking Changes:** 0  

**By Type:**
- Schema Changes: 9
- Data Migrations: 3
- Initial Schema: 1

**By Version:**
- v1.0.0: 1 migration
- v1.1.0: 1 migration
- v1.2.0: 1 migration (formerly labelled; now part of v1.0.0-alpha.6 batch)
- v1.3.0: 1 migration (formerly labelled; now part of v1.0.0-alpha.6 batch)
- v1.0.0-alpha.6: 9 migrations

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

*Last Updated: 2026-03-15*  
*Maintained by: Project Maintainers*  
*Review Frequency: Per Release*

