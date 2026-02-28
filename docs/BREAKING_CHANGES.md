# Pre-Release Migration Reference

> **Note:** This document was written during pre-release development and describes
> internal changes between development milestones (`1.0.0-alpha.1` through `1.0.0-alpha.4`).
> Since the first official release is `1.0.0` (which includes all features below),
> this guide is primarily useful for **early fork users** who cloned the repository
> before multi-tenancy was added.
>
> If you are starting fresh with `1.0.0`, you can ignore this document.

---

# Breaking Changes

This document tracks all breaking changes in the Authentication API, providing clear migration paths and impact assessments.

---

## What is a Breaking Change?

A breaking change is any modification that requires action from users to maintain compatibility:
- Database schema changes requiring migration
- API endpoint modifications
- Configuration/environment variable changes
- Removed or renamed features
- Changed behavior that affects existing integrations

---

## Multi-Tenancy Architecture

### Summary
- All API requests require the `X-App-ID` header
- Database migration required (new tables and schema changes)
- JWT tokens include `app_id` claim (old tokens are invalid)
- OAuth configuration moved from env vars to database (per-application)
- Automatic data migration to default tenant/app
- Rollback scripts provided

---

### API Changes

All API endpoints now require the `X-App-ID` header:

```bash
curl -X POST /auth/register \
  -H "Content-Type: application/json" \
  -H "X-App-ID: 00000000-0000-0000-0000-000000000001" \
  -d '{"email":"user@example.com","password":"secret"}'
```

**Exceptions (no header required):**
- `/swagger/*` - Swagger documentation
- `/admin/*` - Admin endpoints (uses different auth)
- OAuth callbacks (app_id in state parameter)

**Error without header:**
```json
{"error": "X-App-ID header is required"}
```
HTTP Status: `400 Bad Request`

---

### Database Schema Changes

**New Tables:** `tenants`, `applications`, `oauth_provider_configs`

**Modified Tables:** `users`, `social_accounts`, `activity_logs` now include `app_id` foreign key.

**Data Migration:** A default tenant and application (`00000000-0000-0000-0000-000000000001`) are created automatically, and all existing data is migrated to it.

**Email uniqueness** is now scoped per-application (was globally unique).

---

### JWT Token Changes

JWT tokens now include an `app_id` claim. All tokens issued before migration are invalid -- users must re-authenticate.

---

### OAuth Configuration Changes

OAuth credentials moved from environment variables (global) to database (per-application). A migration tool is provided:

```bash
go run cmd/migrate_oauth/main.go
```

Environment variables still work as a fallback for the default application.

---

### Migration Steps

1. **Backup database** (critical)
2. **Apply migration:** `make migrate-up`
3. **Migrate OAuth:** `go run cmd/migrate_oauth/main.go`
4. **Update API clients:** Add `X-App-ID` header to all requests
5. **Notify users:** They must re-login (JWTs invalidated)

### Rollback

```bash
# Restore from backup (fastest)
psql -U postgres -d auth_db < backup.sql

# Or use rollback script
psql -U postgres -d auth_db -f migrations/20260105_add_multi_tenancy_rollback.sql
```

---

## Smart Activity Logging

**Impact:** Non-breaking (backward compatible)

Database schema additions:
- `severity` column on `activity_logs`
- `expires_at` column on `activity_logs`
- `is_anomaly` column on `activity_logs`

All changes are additive with defaults. No code or configuration changes required. See [Activity Logging](activity-logging.md) for details.

---

## Migration Strategy

### For Users

1. Read breaking changes for your target milestone
2. Backup your database
3. Test in development/staging first
4. Apply migrations
5. Update configuration if needed
6. Deploy and verify

### For Contributors

When proposing breaking changes:

1. Document in this file first
2. Provide migration path with rollback scripts
3. Update CHANGELOG.md
4. Follow semver guidelines
5. Wait for maintainer approval

---

## Need Help?

- [Upgrade Guide](migrations/UPGRADE_GUIDE.md)
- [Migration System](migrations/MIGRATIONS.md)
- [GitHub Issues](https://github.com/gjovanovicst/auth_api/issues)
