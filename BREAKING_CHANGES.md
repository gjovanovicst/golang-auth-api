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

## Current Version: v1.1.0

### Summary
- ✅ All changes backward compatible
- ✅ Smart Activity Logging added (opt-in)
- ✅ Zero breaking changes
- ✅ Safe to upgrade from v1.0.0

---

## Version History

## [v1.1.0] - 2024-01-03

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

*Last Updated: 2024-01-03*
*Next Review: 2024-04-03*

