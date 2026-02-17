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

## Current Version: v2.0.0

### Summary
- 🚨 **MAJOR BREAKING CHANGES** - Multi-Tenancy Architecture
- ⚠️ **Requires API client updates** - All requests need `X-App-ID` header
- ⚠️ **Database migration required** - New tables and schema changes
- ⚠️ **JWT tokens invalidated** - Users must re-authenticate
- ⚠️ **OAuth configuration changes** - Credentials now per-application
- ✅ **Automatic data migration** - Existing data migrated to default tenant/app
- ✅ **Rollback available** - Safe rollback scripts provided

---

## Version History

## [v2.0.0] - 2026-01-19

### 🚨 BREAKING CHANGE: Multi-Tenancy Architecture

**Type:** Major Breaking Change (API + Database + Configuration)

**Impact:** 🔴 **HIGH** - All API clients must be updated

#### What Changed

**1. API Changes (BREAKING)**

**ALL API endpoints now require `X-App-ID` header:**

```bash
# ❌ BEFORE (v1.x) - This will FAIL in v2.0
curl -X POST /auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"user@example.com","password":"secret"}'

# ✅ AFTER (v2.x) - This is REQUIRED
curl -X POST /auth/register \
  -H "Content-Type: application/json" \
  -H "X-App-ID: 00000000-0000-0000-0000-000000000001" \
  -d '{"email":"user@example.com","password":"secret"}'
```

**Exceptions (no header required):**
- `/swagger/*` - Swagger documentation
- `/admin/*` - Admin endpoints (uses different auth)
- OAuth callbacks (app_id in state parameter)

**Error Response Without Header:**
```json
{
  "error": "X-App-ID header is required"
}
```
HTTP Status: `400 Bad Request`

**2. Database Schema Changes (BREAKING)**

**New Tables:**
```sql
-- Tenant organizations
CREATE TABLE tenants (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE,
    updated_at TIMESTAMP WITH TIME ZONE
);

-- Applications per tenant
CREATE TABLE applications (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    name TEXT NOT NULL,
    description TEXT,
    created_at TIMESTAMP WITH TIME ZONE,
    updated_at TIMESTAMP WITH TIME ZONE
);

-- OAuth configuration per application
CREATE TABLE oauth_provider_configs (
    id UUID PRIMARY KEY,
    app_id UUID NOT NULL REFERENCES applications(id),
    provider TEXT NOT NULL,
    client_id TEXT NOT NULL,
    client_secret TEXT NOT NULL,
    redirect_url TEXT NOT NULL,
    is_enabled BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP WITH TIME ZONE,
    updated_at TIMESTAMP WITH TIME ZONE,
    UNIQUE(app_id, provider)
);
```

**Modified Tables:**
```sql
-- All existing tables now have app_id
ALTER TABLE users ADD COLUMN app_id UUID NOT NULL REFERENCES applications(id);
ALTER TABLE social_accounts ADD COLUMN app_id UUID NOT NULL REFERENCES applications(id);
ALTER TABLE activity_logs ADD COLUMN app_id UUID NOT NULL REFERENCES applications(id);

-- Email uniqueness now scoped per application (was globally unique)
DROP INDEX idx_users_email;
CREATE UNIQUE INDEX idx_email_app_id ON users(email, app_id);
```

**Data Migration:**
- Default tenant created: `00000000-0000-0000-0000-000000000001`
- Default application created: `00000000-0000-0000-0000-000000000001`
- All existing users, social accounts, and logs migrated to default app
- Email uniqueness constraint updated (same email can exist in different apps)

**3. JWT Token Changes (BREAKING)**

**JWT tokens now include `app_id` claim:**
```json
{
  "user_id": "123",
  "email": "user@example.com",
  "app_id": "00000000-0000-0000-0000-000000000001",  // NEW
  "exp": 1234567890
}
```

**Impact:**
- ❌ All existing JWT tokens (access + refresh) are INVALID after migration
- ✅ Users must re-authenticate to get new tokens
- ✅ Token validation checks `app_id` matches request header

**4. OAuth Configuration Changes (BREAKING)**

**Before (v1.x):** Environment variables (global)
```bash
# .env
GOOGLE_CLIENT_ID=xxx
GOOGLE_CLIENT_SECRET=xxx
FACEBOOK_CLIENT_ID=xxx
FACEBOOK_CLIENT_SECRET=xxx
GITHUB_CLIENT_ID=xxx
GITHUB_CLIENT_SECRET=xxx
```

**After (v2.x):** Database configuration (per-application)
```bash
# Use migration tool to transfer env vars to database:
go run cmd/migrate_oauth/main.go

# OR manually via Admin API:
POST /admin/oauth-providers
{
  "app_id": "00000000-0000-0000-0000-000000000001",
  "provider": "google",
  "client_id": "xxx",
  "client_secret": "xxx",
  "redirect_url": "http://localhost:8080/auth/google/callback",
  "is_enabled": true
}
```

**Legacy Support:**
- Environment variables still work for default app (`00000000-0000-0000-0000-000000000001`)
- Database config takes precedence over environment variables
- Migration tool provided: `cmd/migrate_oauth/main.go`

**5. New Admin API Endpoints**

**Tenant Management:**
- `POST /admin/tenants` - Create tenant
- `GET /admin/tenants` - List tenants (paginated)

**Application Management:**
- `POST /admin/apps` - Create application
- `GET /admin/apps` - List applications (paginated)

**OAuth Provider Management:**
- `POST /admin/oauth-providers` - Configure OAuth provider
- `GET /admin/oauth-providers/:app_id` - List providers for app
- `PUT /admin/oauth-providers/:id` - Update provider config
- `DELETE /admin/oauth-providers/:id` - Delete provider config

#### Migration Required

**YES** - Database migration is MANDATORY

**Migration Files:**
- Up: `migrations/20260105_add_multi_tenancy.sql`
- Down: `migrations/20260105_add_multi_tenancy_rollback.sql`

**Apply Migration:**
```bash
# Recommended: Use Makefile
make migrate-backup    # Create backup first
make migrate-up        # Apply migration

# OR manual:
pg_dump -U postgres -d auth_db > backup_$(date +%Y%m%d).sql
psql -U postgres -d auth_db -f migrations/20260105_add_multi_tenancy.sql
```

**Migration Steps:**
1. **Backup Database** - CRITICAL, do not skip!
2. **Apply SQL migration** - Creates tables, migrates data
3. **Migrate OAuth credentials** - Run `go run cmd/migrate_oauth/main.go`
4. **Update API clients** - Add `X-App-ID` header to all requests
5. **Notify users** - Force re-authentication (JWTs invalid)
6. **Test thoroughly** - Verify all endpoints work

**Estimated Downtime:** 5-15 minutes (depends on database size)

#### Is This Breaking?

**YES** - This is a MAJOR breaking change:
- ❌ API clients without `X-App-ID` header will FAIL with 400 error
- ❌ Existing JWT tokens will be INVALID (must re-authenticate)
- ❌ OAuth configuration must be migrated from env vars to database
- ❌ Database schema changes require migration (new tables + columns)
- ❌ Email uniqueness behavior changed (now scoped per app)

#### Who Is Affected?

**All users and integrations:**
- ✅ **Web/Mobile Apps** - Must add `X-App-ID` header to all API calls
- ✅ **Third-party Integrations** - Must update API client code
- ✅ **End Users** - Must re-login after migration (JWTs invalid)
- ✅ **Administrators** - Must migrate OAuth configuration
- ✅ **Database** - Migration required (automatic data migration)

#### Migration Path

**Step-by-Step Guide:**

**1. Pre-Migration (Before Upgrade)**
```bash
# Backup database
make migrate-backup

# Review migration script
cat migrations/20260105_add_multi_tenancy.sql

# Test in staging environment first
```

**2. Migration (During Upgrade)**
```bash
# Stop application
systemctl stop auth-api  # or docker-compose down

# Apply database migration
make migrate-up

# Migrate OAuth credentials from env vars to database
go run cmd/migrate_oauth/main.go

# Verify migration success
psql -U postgres -d auth_db -c "SELECT * FROM tenants;"
psql -U postgres -d auth_db -c "SELECT * FROM applications;"
```

**3. Update Configuration**
```bash
# Optional: Remove OAuth credentials from .env (now in database)
# Keep them as fallback or remove after testing

# Update API clients to include X-App-ID header
# Default app ID: 00000000-0000-0000-0000-000000000001
```

**4. Start Application**
```bash
# Start with new version
systemctl start auth-api  # or docker-compose up -d

# Monitor logs for errors
tail -f /var/log/auth-api/app.log

# Test critical endpoints
curl -X POST /auth/register \
  -H "X-App-ID: 00000000-0000-0000-0000-000000000001" \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","password":"testpass123"}'
```

**5. Post-Migration**
```bash
# Notify users to re-login (JWTs invalid)
# Update API documentation
# Monitor application logs
# Keep database backup for at least 7 days
```

#### Rollback

**If migration fails or issues arise:**

```bash
# Stop application
systemctl stop auth-api

# Restore from backup (fastest)
psql -U postgres -d auth_db < backup_20260119.sql

# OR use rollback script
psql -U postgres -d auth_db -f migrations/20260105_add_multi_tenancy_rollback.sql

# Downgrade to v1.1.0
git checkout v1.1.0
make build
systemctl start auth-api
```

**⚠️ WARNING:** Rollback will:
- Remove all tenants and applications created after migration
- Revert to single-tenant mode
- Restore global email uniqueness constraint
- Existing JWTs still invalid (users must re-login)

#### Testing Checklist

Before deploying to production:

- [ ] Backup created and verified
- [ ] Migration tested in staging environment
- [ ] All API endpoints tested with `X-App-ID` header
- [ ] User registration works
- [ ] User login works (generates new JWTs with `app_id`)
- [ ] Social login works (OAuth credentials migrated)
- [ ] 2FA works
- [ ] Profile endpoints work
- [ ] Activity logs created with `app_id`
- [ ] Admin API endpoints accessible
- [ ] Rollback tested successfully
- [ ] API documentation updated
- [ ] Users notified about re-authentication requirement

#### Benefits

**Why this breaking change?**

- 🏢 **Multi-Tenancy** - Serve multiple organizations/clients from one deployment
- 🔒 **Data Isolation** - Complete tenant/app data separation at database level
- 🔐 **Per-App OAuth** - Different OAuth credentials per application
- 📊 **Better Analytics** - Track usage per tenant/application
- 💰 **SaaS-Ready** - Build multi-tenant SaaS products
- ⚡ **Resource Efficiency** - One deployment serves many apps
- 🔧 **Flexible Configuration** - OAuth and settings per application

#### Documentation

- **Migration Guide**: `migrations/20260105_add_multi_tenancy.md`
- **Migration SQL**: `migrations/20260105_add_multi_tenancy.sql`
- **Rollback SQL**: `migrations/20260105_add_multi_tenancy_rollback.sql`
- **OAuth Migration Tool**: `cmd/migrate_oauth/main.go`
- **API Documentation**: Swagger at `/swagger/index.html`
- **Changelog**: `CHANGELOG.md` (v2.0.0 section)

#### Support

**Need help with migration?**
- 📖 Read [UPGRADE_GUIDE.md](UPGRADE_GUIDE.md) for detailed instructions
- 📖 Read [MIGRATIONS.md](docs/migrations/MIGRATIONS.md)
- 🐛 Check GitHub issues: https://github.com/gjovanovicst/auth_api/issues
- 💬 Open new issue with "migration-help" label
- 📧 Contact maintainers

---

## [v1.1.0] - 2024-12-04

### Added: Smart Activity Logging System

**Type:** Non-Breaking (Opt-in Enhancement)

**Impact:** ⚠️ Database Migration Required (Backward Compatible)

#### What Changed

**Database Schema:**
- Added `severity` column to `activity_logs` (VARCHAR, NOT NULL, DEFAULT 'INFORMATIONAL')
- Added `expires_at` column to `activity_logs` (TIMESTAMP WITH TIME ZONE, NULLABLE)
- Added `is_anomaly` column to `activity_logs` (BOOLEAN, NOT NULL, DEFAULT false)
- Added indexes: `idx_activity_logs_expires`, `idx_activity_logs_cleanup`, `idx_activity_logs_user_timestamp`

**New Features:**
- Event severity classification (Critical/Important/Informational)
- Anomaly detection for conditional logging
- Automatic log retention and cleanup
- Configurable logging via environment variables

**Configuration (Optional):**
New environment variables available (all optional):
```bash
LOG_CLEANUP_ENABLED=true
LOG_CLEANUP_INTERVAL=24h
LOG_ANOMALY_DETECTION_ENABLED=true
LOG_RETENTION_CRITICAL=365
LOG_RETENTION_IMPORTANT=180
LOG_RETENTION_INFORMATIONAL=90
```

#### Migration Required

**Yes** - Database schema changes required

**Migration File:** `migrations/20240103_add_activity_log_smart_fields.sql`

**Apply Migration:**
```bash
# Docker/Development
psql -U postgres -d auth_db -f migrations/20240103_add_activity_log_smart_fields.sql

# OR start app - existing logs are updated automatically
make docker-dev
```

#### Is This Breaking?

**NO** - This is backward compatible:
- ✅ All existing API endpoints work unchanged
- ✅ Existing logs automatically updated with defaults
- ✅ No code changes required
- ✅ No configuration changes required
- ✅ Works with zero configuration

#### Should You Migrate?

**Recommended but not required immediately**

**Benefits:**
- 80-95% reduction in database size
- Automatic log cleanup
- Better performance
- Anomaly detection

**Migration Path:**
1. Backup database
2. Apply migration SQL
3. Restart application
4. (Optional) Configure via environment variables

#### Rollback

If needed, rollback is available:
```bash
psql -U postgres -d auth_db -f migrations/20240103_add_activity_log_smart_fields_rollback.sql
```

**Note:** Rollback will remove new fields but keep existing data.

#### Documentation

- [Smart Logging Guide](docs/ACTIVITY_LOGGING_GUIDE.md)
- [Migration Guide](migrations/README_SMART_LOGGING.md)
- [Quick Setup](docs/QUICK_SETUP_LOGGING.md)
- [Environment Variables](docs/ENV_VARIABLES.md)

---

## [v1.0.0] - Initial Release

### Initial Features
- User registration and authentication
- JWT access and refresh tokens
- Social login (Google, Facebook, GitHub)
- Email verification
- Password reset
- Two-factor authentication (2FA)
- Activity logging (basic)
- Redis integration
- Swagger documentation

**Breaking Changes:** None (initial release)

---

## Upcoming Changes (Planned)

### v1.2.0 (Planned)

**Potential Changes:**
- Role-based access control (RBAC)
- Admin panel endpoints
- API rate limiting

**Expected Breaking Changes:** TBD

**Status:** Under discussion

---

## Migration Strategy

### For Users

**Before Upgrading:**
1. Read breaking changes for your target version
2. Check migration requirements
3. Backup your database
4. Test in development/staging first
5. Plan downtime if needed

**During Upgrade:**
1. Stop application
2. Backup database
3. Apply migrations
4. Update configuration (if needed)
5. Deploy new version
6. Verify functionality
7. Monitor logs

**After Upgrade:**
1. Test critical functionality
2. Monitor application logs
3. Monitor database performance
4. Keep backup for 7 days minimum

### For Contributors

**When Making Breaking Changes:**
1. Document in this file FIRST
2. Provide migration path
3. Add to UPGRADE_GUIDE.md
4. Update CHANGELOG.md
5. Bump version appropriately (semver)
6. Add migration scripts
7. Add rollback scripts
8. Update documentation
9. Add tests for migration

**Semver Guidelines:**
- **Major version (2.0.0):** Breaking API changes
- **Minor version (1.1.0):** New features, backward compatible
- **Patch version (1.0.1):** Bug fixes, backward compatible

---

## Breaking Change Categories

### 1. Database Schema Changes

**Examples:**
- Renaming columns/tables
- Changing column types
- Adding NOT NULL columns without defaults
- Removing columns/tables
- Changing constraints

**Required:**
- Migration SQL (up + down)
- Data migration scripts (if needed)
- Testing on copy of production data

### 2. API Endpoint Changes

**Examples:**
- Changing endpoint URLs
- Modifying request/response format
- Removing endpoints
- Changing authentication requirements

**Required:**
- API versioning (consider `/v2/` prefix)
- Deprecation notice (minimum 1 version)
- Updated Swagger documentation
- Client library updates

### 3. Configuration Changes

**Examples:**
- Renamed environment variables
- Required new environment variables
- Changed configuration format
- Removed configuration options

**Required:**
- Update .env.example
- Backward compatibility check
- Default values when possible
- Migration guide in ENV_VARIABLES.md

### 4. Behavior Changes

**Examples:**
- Changed authentication flow
- Modified token expiration
- Altered logging behavior
- Changed error responses

**Required:**
- Clear documentation of changes
- Reason for change
- New expected behavior
- Impact assessment

### 5. Dependency Changes

**Examples:**
- Major version bumps of dependencies
- Removed dependencies
- Changed minimum version requirements

**Required:**
- Document new requirements
- Update go.mod
- Test compatibility
- Update installation docs

---

## Deprecation Policy

### Deprecation Process

**1. Announce (Version N)**
- Mark feature as deprecated in docs
- Add deprecation warnings in code
- Provide alternative/migration path

**2. Support (Version N+1)**
- Feature still works but deprecated
- Warnings in logs
- Documentation clearly marked

**3. Remove (Version N+2)**
- Feature removed
- Breaking change documented
- Migration guide provided

**Example:**
```
v1.0.0: Feature X works normally
v1.1.0: Feature X deprecated, use Feature Y instead (warnings added)
v1.2.0: Feature X still works (final warning)
v2.0.0: Feature X removed (breaking change)
```

### Current Deprecations

**None at this time**

---

## FAQ

### Q: How do I know if a change affects me?

**A:** Check the "Impact" section for each change. If it mentions something you use, read the full details.

### Q: Can I skip versions?

**A:** Yes, but apply ALL migrations between versions in order. See [UPGRADE_GUIDE.md](UPGRADE_GUIDE.md).

### Q: What if migration fails?

**A:** Use the provided rollback script and restore from backup. See [MIGRATIONS.md](docs/migrations/MIGRATIONS.md#rollback-process).

### Q: How long are old versions supported?

**A:** Current policy:
- Latest version: Full support
- Previous version: Security fixes only (6 months)
- Older versions: No support (upgrade recommended)

### Q: Can I contribute breaking changes?

**A:** Yes! See [CONTRIBUTING.md](CONTRIBUTING.md) for the process. Breaking changes require:
- Strong justification
- Migration path
- Full documentation
- Maintainer approval

---

## Version Compatibility Matrix

| App Version | Database Schema | Min Go Version | Min PostgreSQL | Breaking Changes |
|-------------|-----------------|----------------|----------------|------------------|
| v1.0.0 | v1.0.0 | 1.21+ | 12+ | - |
| v1.1.0 | v1.1.0 | 1.21+ | 12+ | None |
| v2.0.0 | v2.0.0 | 1.23+ | 12+ | **YES** - Multi-tenancy (API + DB) |

**Upgrade Paths:**
- v1.0.0 → v2.0.0: Apply both v1.1.0 and v2.0.0 migrations in order
- v1.1.0 → v2.0.0: Apply v2.0.0 migration only
- v2.0.0 → v1.1.0: Rollback available (data loss possible)

---

## Need Help?

- 📖 Read [UPGRADE_GUIDE.md](UPGRADE_GUIDE.md) for step-by-step instructions
- 📖 Read [MIGRATIONS.md](docs/migrations/MIGRATIONS.md) for migration help
- 🐛 Check existing GitHub issues
- 💬 Open a new issue with "breaking change" label
- 📧 Contact maintainers

---

## Contributing

When proposing breaking changes:
1. Open an issue first for discussion
2. Provide clear justification
3. Document migration path
4. Follow the template in [CONTRIBUTING.md](CONTRIBUTING.md)
5. Wait for maintainer approval before implementing

**Template:** See [migrations/TEMPLATE.md](migrations/TEMPLATE.md)

---

*Last Updated: 2026-01-19*
*Next Review: 2026-04-19*


