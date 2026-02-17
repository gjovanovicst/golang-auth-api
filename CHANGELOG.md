# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [2.0.0] - 2026-01-19

### 🚨 BREAKING CHANGES - Multi-Tenancy Support

This release introduces **multi-tenancy** architecture, enabling the API to serve multiple tenants and applications. **This is a major breaking change** that requires API client updates.

#### Required API Changes

**All API requests now require the `X-App-ID` header:**

```bash
# Before (v1.x)
curl -X POST /auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"user@example.com","password":"secret"}'

# After (v2.x)
curl -X POST /auth/register \
  -H "Content-Type: application/json" \
  -H "X-App-ID: 00000000-0000-0000-0000-000000000001" \
  -d '{"email":"user@example.com","password":"secret"}'
```

**Exceptions (no `X-App-ID` required):**
- Swagger documentation endpoints (`/swagger/*`)
- Admin endpoints (`/admin/*`)
- OAuth callback endpoints (app_id in state parameter)

#### Migration Impact

**Database Schema:**
- New tables: `tenants`, `applications`, `oauth_provider_configs`
- Modified tables: `users`, `social_accounts`, `activity_logs` now include `app_id` foreign key
- Existing data automatically migrated to default tenant/application (`00000000-0000-0000-0000-000000000001`)
- Email uniqueness now scoped per application (was globally unique)

**JWT Tokens:**
- JWT tokens now include `app_id` claim
- Existing tokens issued before upgrade will be invalid
- Users must re-authenticate after migration

**OAuth Configuration:**
- OAuth provider credentials (Google, Facebook, GitHub) now configured per-application
- Use migration tool `cmd/migrate_oauth/main.go` to migrate existing credentials from environment variables to database
- Legacy environment-based OAuth config still supported for default app

### Added

#### Multi-Tenancy Architecture
- **Tenant Management**: Create and manage multiple tenant organizations via admin API
- **Application Management**: Each tenant can have multiple applications with isolated user bases
- **Per-Application OAuth**: OAuth providers (Google, Facebook, GitHub) configured per-application in database
- **Admin API Endpoints**:
  - `POST /admin/tenants` - Create tenant
  - `GET /admin/tenants` - List tenants (paginated)
  - `POST /admin/apps` - Create application
  - `GET /admin/apps` - List applications (paginated)
  - `POST /admin/oauth-providers` - Configure OAuth provider for app
  - `GET /admin/oauth-providers/:app_id` - List OAuth providers for app
  - `PUT /admin/oauth-providers/:id` - Update OAuth provider config
  - `DELETE /admin/oauth-providers/:id` - Delete OAuth provider config

#### New Middleware
- **AppID Middleware**: Validates and injects `X-App-ID` header into request context
- Query parameter fallback: `?app_id=<uuid>` supported when header not available

#### New Models
- **Tenant Model** (`pkg/models/tenant.go`): Organization-level entity
- **Application Model** (`pkg/models/application.go`): Tenant's application entity
- **OAuthProviderConfig Model** (`pkg/models/oauth_provider_config.go`): Per-app OAuth credentials

#### Migration Tools
- **Migration Script**: `migrations/20260105_add_multi_tenancy.sql` with automatic data migration
- **Rollback Script**: `migrations/20260105_add_multi_tenancy_rollback.sql` for safe rollback
- **OAuth Migration Tool**: `cmd/migrate_oauth/main.go` to migrate OAuth credentials from env to database
- **Enhanced Backup Scripts**: 
  - `scripts/backup_db.sh` (Unix/Mac)
  - `scripts/backup_db.bat` (Windows)
- **Migration Helper Scripts**:
  - `scripts/apply_pending_migrations.sh` - Apply pending migrations
  - `scripts/rollback_last_migration.sh` - Rollback last migration

#### Documentation
- **Copilot Instructions**: `.github/copilot-instructions.md` for AI-assisted development
- **Migration Documentation**: `migrations/20260105_add_multi_tenancy.md`
- **Admin API DTOs**: `pkg/dto/admin.go` with request/response structures
- **Updated Swagger**: Complete API documentation with new admin endpoints

### Changed

#### Authentication Flow
- JWT generation now includes `app_id` claim (see `pkg/jwt/jwt.go`)
- Auth middleware validates `app_id` from token against request header
- User lookup queries now scoped by `app_id`

#### Social Login
- OAuth state now includes `app_id` for callback routing
- Social account linkage scoped per application
- OAuth credentials loaded from database per-application (fallback to env vars for default app)

#### Two-Factor Authentication
- TOTP secrets and recovery codes now scoped per application
- 2FA state isolated between applications

#### User Management
- User registration scoped by `app_id`
- Email uniqueness constraint now `(email, app_id)` instead of global
- Profile endpoints return data scoped to request's `app_id`

#### Activity Logging
- All activity logs include `app_id` for audit trail segmentation
- Log queries filtered by application context

#### Testing
- All API tests updated to include `X-App-ID` header
- Test scripts (`test_api.sh`, `test_logout.sh`) updated with multi-tenancy support

#### Configuration
- Redis key prefixes now include `app_id` for session isolation
- CORS middleware allows `X-App-ID` header in requests

### Fixed
- Docker network creation documentation (`README.md`) - Added step to create shared network before starting containers

### Security
- **Data Isolation**: Complete tenant/application data isolation at database level
- **OAuth Security**: OAuth credentials stored per-application with encrypted secrets
- **JWT Claims**: App ID validation prevents cross-application token reuse
- **Index Updates**: Optimized indexes for multi-tenant queries

### Migration Guide

#### For Existing Installations (Upgrading from v1.x)

**⚠️ CRITICAL: Backup your database before proceeding!**

1. **Backup Database**:
   ```bash
   make migrate-backup
   # or manually:
   pg_dump -U postgres -d auth_db > backup_$(date +%Y%m%d_%H%M%S).sql
   ```

2. **Apply Migration**:
   ```bash
   make migrate-up
   # This automatically:
   # - Creates tenants, applications, oauth_provider_configs tables
   # - Adds app_id columns to users, social_accounts, activity_logs
   # - Migrates all existing data to default tenant/app
   # - Updates indexes (email uniqueness now per-app)
   # - Records migration in schema_migrations table
   ```

3. **Migrate OAuth Credentials** (if using social login):
   ```bash
   go run cmd/migrate_oauth/main.go
   # Reads from .env and creates oauth_provider_configs entries
   # For providers: Google, Facebook, GitHub
   ```

4. **Update API Clients**:
   - Add `X-App-ID: 00000000-0000-0000-0000-000000000001` header to all requests
   - Default app ID is created automatically during migration
   - Update documentation/SDKs with new header requirement
   - Notify users to re-authenticate (existing JWTs are invalid)

5. **Test Endpoints**:
   ```bash
   # Test registration
   curl -X POST http://localhost:8080/auth/register \
     -H "X-App-ID: 00000000-0000-0000-0000-000000000001" \
     -H "Content-Type: application/json" \
     -d '{"email":"test@example.com","password":"Test123!@#"}'
   
   # Test login
   curl -X POST http://localhost:8080/auth/login \
     -H "X-App-ID: 00000000-0000-0000-0000-000000000001" \
     -H "Content-Type: application/json" \
     -d '{"email":"test@example.com","password":"Test123!@#"}'
   ```

6. **Rollback (if needed)**:
   ```bash
   # If migration fails or issues arise:
   make migrate-down
   # or manually:
   psql -U postgres -d auth_db -f migrations/20260105_add_multi_tenancy_rollback.sql
   
   # Restore from backup if necessary:
   psql -U postgres -d auth_db < backup_20260119_143022.sql
   ```

**Estimated Migration Time:**
- Small databases (<10k users): 1-2 minutes
- Medium databases (10k-100k users): 3-5 minutes
- Large databases (>100k users): 5-15 minutes

**Expected Downtime:** 5-15 minutes (application must be stopped during migration)

#### For New Installations

- Multi-tenancy enabled by default
- Default tenant and application created automatically
- Use admin API to create additional tenants/applications:
  ```bash
  # Create tenant
  POST /admin/tenants
  {"name": "Acme Corp"}
  
  # Create application
  POST /admin/apps
  {"tenant_id": "<tenant_id>", "name": "Mobile App", "description": "iOS/Android app"}
  
  # Configure OAuth for app
  POST /admin/oauth-providers
  {
    "app_id": "<app_id>",
    "provider": "google",
    "client_id": "xxx",
    "client_secret": "yyy",
    "redirect_url": "https://app.example.com/auth/google/callback",
    "is_enabled": true
  }
  ```

#### Troubleshooting

**Issue: "X-App-ID header is required" error**
- Solution: Add header to all API requests (except /swagger/* and /admin/*)
- Default app ID: `00000000-0000-0000-0000-000000000001`

**Issue: JWT tokens not working after migration**
- Solution: This is expected. Users must re-login to get new JWTs with `app_id` claim

**Issue: Social login not working**
- Solution: Run OAuth migration tool: `go run cmd/migrate_oauth/main.go`
- Or configure via Admin API: `POST /admin/oauth-providers`

**Issue: Email already exists error for different apps**
- Solution: This is correct behavior - email uniqueness is now per-app, not global

**Issue: Migration fails with foreign key error**
- Solution: Check database constraints. Rollback and restore from backup.

**Need Help?**
- See: [BREAKING_CHANGES.md](BREAKING_CHANGES.md) for detailed migration guide
- See: `migrations/20260105_add_multi_tenancy.md` for technical details
- Open GitHub issue with "migration-help" label

---

## [1.1.0] - 2024-12-04

### Added

#### CI/CD Improvements
- **GitHub Actions Workflow**: Complete CI/CD pipeline with test, build, and security-scan jobs
- **Local Testing Support**: Full compatibility with `act` for running GitHub Actions locally
- **Improved Port Configuration**: Services use non-conflicting ports (PostgreSQL: 5435, Redis: 6381) for CI
- **Smart Artifact Handling**: Conditional artifact upload/download based on environment (skips for local act runs)

#### Security Enhancements
- **Gosec Security Scanner**: Automated security scanning integrated into CI/CD
- **Nancy Vulnerability Scanner**: Optional dependency vulnerability scanning (requires authentication)
- **Security Exception Documentation**: Proper `#nosec` comments with justification for legitimate cases

#### Test Infrastructure
- **Environment Variable Support**: Tests now properly read from CI environment variables via `viper.AutomaticEnv()`
- **Redis Connection Handling**: Improved test reliability with proper Redis configuration
- **Test Coverage**: Maintained high test coverage across all components

#### Documentation
- **CI/CD Commands Section**: Added commands for running CI locally with act
- **Updated README**: Added CI/CD features to Developer Experience section
- **Installation Guide**: Added note about installing act for local CI testing

### Fixed
- Port conflicts in CI/CD workflow when running with `act` (changed from default 5432/6379 to 5435/6381)
- Test configuration to respect environment variables via `viper.AutomaticEnv()` and `viper.SetDefault()`
- Artifact operations now skip when running locally with `act` using `if: ${{ !env.ACT }}` condition
- Security scanner false positive for non-cryptographic random number usage in log sampling
- Nancy vulnerability scanner now uses `continue-on-error` to prevent CI failures when OSS Index authentication is not configured

### Changed
- CI workflow now uses different ports to avoid conflicts with local development environments
- Test setup uses `SetDefault` instead of `Set` to allow environment variable override

### Added - Professional Activity Logging System

#### Migration System & Documentation

#### Smart Logging with 80-95% Database Reduction
- **Event Severity Classification**: Events categorized as Critical, Important, or Informational
- **Intelligent Logging**: High-frequency events (TOKEN_REFRESH, PROFILE_ACCESS) disabled by default
- **Anomaly Detection**: Automatically logs unusual patterns (new IP, new device) even for disabled events
- **Automatic Cleanup**: Background service removes expired logs based on retention policies
- **Configurable Retention**: 
  - Critical events: 365 days (LOGIN, PASSWORD_CHANGE, 2FA changes)
  - Important events: 180 days (EMAIL_VERIFY, SOCIAL_LOGIN, PROFILE_UPDATE)
  - Informational events: 90 days (TOKEN_REFRESH, PROFILE_ACCESS when enabled)

#### New Configuration System
- Created `internal/config/logging.go` for centralized logging configuration
- All settings configurable via environment variables
- Event enable/disable controls per event type
- Sampling rates for high-frequency events
- Anomaly detection configuration
- Retention policies per severity level

#### Anomaly Detection Engine
- Created `internal/log/anomaly.go` for behavioral analysis
- Detects new IP addresses from user's historical patterns
- Detects new devices/browsers (user agent changes)
- Configurable pattern analysis window (default: 30 days)
- Optional unusual time access detection
- Privacy-preserving pattern storage (hashed IPs/user agents)

#### Enhanced Data Model
- Added `severity` field to activity_logs (CRITICAL, IMPORTANT, INFORMATIONAL)
- Added `expires_at` field for automatic expiration timestamps
- Added `is_anomaly` flag to identify anomaly-triggered logs
- Created composite indexes for efficient queries and cleanup
- Migration script with rollback capability

#### Comprehensive Migration System
- **[MIGRATIONS.md](docs/migrations/MIGRATIONS.md)**: User-friendly migration guide with step-by-step instructions
- **[BREAKING_CHANGES.md](BREAKING_CHANGES.md)**: Breaking changes tracker with version history
- **[UPGRADE_GUIDE.md](UPGRADE_GUIDE.md)**: Detailed version upgrade instructions with rollback procedures
- **[migrations/README.md](migrations/README.md)**: Developer-focused migration guide with best practices
- **[migrations/TEMPLATE.md](migrations/TEMPLATE.md)**: Standardized template for creating new migrations
- **[migrations/MIGRATIONS_LOG.md](migrations/MIGRATIONS_LOG.md)**: Historical log of all applied migrations

#### Migration Tools
- **scripts/migrate.sh**: Interactive Unix/Mac migration tool with:
  - Migration status checking
  - Automatic backups before migrations
  - Apply/rollback functionality
  - Database connection testing
- **scripts/migrate.bat**: Windows-compatible migration tool
- **Makefile targets**:
  - `make migrate` - Interactive migration tool
  - `make migrate-up` - Apply migrations
  - `make migrate-down` - Rollback migrations
  - `make migrate-status` - Check migration status
  - `make migrate-backup` - Create database backup

#### Contributor-Friendly Process
- Updated [CONTRIBUTING.md](CONTRIBUTING.md) with detailed migration guidelines
- Clear process for creating migrations with checklists
- Breaking change documentation requirements
- Testing and verification procedures
- Semver guidelines for version bumping

#### Automatic Cleanup Service
- Created `internal/log/cleanup.go` for background log deletion
- Runs on configurable schedule (default: daily)
- Batch processing to avoid database locks (default: 1000 per batch)
- Graceful shutdown handling
- Statistics tracking and manual trigger capability
- GDPR compliance support (delete user logs on request)

#### Comprehensive Documentation
- Created `docs/ACTIVITY_LOGGING_GUIDE.md` - Complete configuration guide
- Created `docs/ENV_VARIABLES.md` - All environment variables reference
- Created `docs/SMART_LOGGING_IMPLEMENTATION.md` - Implementation summary
- Created `migrations/README_SMART_LOGGING.md` - Migration instructions
- Updated `docs/API.md` with new event categorization
- Updated `README.md` with smart logging features

#### Configuration Examples
```bash
# Default behavior (zero configuration needed)
# - Critical/Important events: Always logged
# - TOKEN_REFRESH/PROFILE_ACCESS: Disabled (logged only on anomaly)
# - Automatic cleanup: Enabled (runs daily)
# - Anomaly detection: Enabled

# Optional customization via environment variables:
LOG_DISABLED_EVENTS=TOKEN_REFRESH,PROFILE_ACCESS
LOG_ANOMALY_DETECTION_ENABLED=true
LOG_RETENTION_CRITICAL=365
LOG_RETENTION_IMPORTANT=180
LOG_RETENTION_INFORMATIONAL=90
LOG_CLEANUP_ENABLED=true
LOG_CLEANUP_INTERVAL=24h
```

#### Expected Impact
- **Database Size**: 80-95% reduction in log volume
- **Performance**: Maintained (async logging) with enhanced indexes
- **Security**: Improved focus on actionable events with anomaly detection
- **Compliance**: Maintained for all critical audit requirements
- **Flexibility**: Fully configurable per deployment environment

#### Breaking Changes
- None - backward compatible with existing activity logs
- Existing logs automatically assigned severity and expiration on migration
- All existing API endpoints continue to work unchanged

#### Migration Required
- Run migration: `migrations/20240103_add_activity_log_smart_fields.sql`
- Rollback available: `migrations/20240103_add_activity_log_smart_fields_rollback.sql`
- See `migrations/README_SMART_LOGGING.md` for detailed instructions

### Added - Profile Sync on Social Login (2025-11-08)

#### Automatic Profile Synchronization
- **Profile data now automatically syncs** from social providers on every login
- System updates both `social_accounts` and `users` tables with latest provider data
- **Smart update strategy**: Only updates fields that have changed
- **Non-blocking**: Authentication succeeds even if profile update fails
- Supports all providers: Google, Facebook, GitHub

#### What Gets Synced
- Profile picture (avatar/photo URL)
- Full name, first name, last name
- Email from provider
- Locale/language preference
- Username (GitHub login, etc.)
- Complete raw provider response (JSONB)
- OAuth access token

#### Benefits
- Users see updated profile pictures immediately after changing them on social platforms
- Name changes on social accounts automatically reflected in app
- No manual sync or refresh needed
- Data stays current with social provider

#### Repository Enhancement
- Added `UpdateSocialAccount()` method to social repository
- Enables full social account record updates via GORM

#### Profile Endpoint Enhancement
- Updated `UserResponse` DTO to include all new profile fields
- Added `SocialAccountResponse` DTO for social account data
- Modified `GetUserByID` repository to preload social accounts
- Enhanced `GetProfile` handler to return complete user profile with social accounts
- Profile endpoint now returns: name, first_name, last_name, profile_picture, locale, social_accounts
- Regenerated Swagger documentation to reflect new profile structure

### Added - Social Login Data Enhancement (2025-11-08)

#### User Model Enhancements
- Added `Name` field to store full name from social login or user input
- Added `FirstName` field for first name from social login
- Added `LastName` field for last name from social login
- Added `ProfilePicture` field to store profile picture URL from social providers
- Added `Locale` field for user's language/locale preference

#### Social Account Model Enhancements
- Added `Email` field to store email from social provider
- Added `Name` field to store name from social provider
- Added `FirstName` field for first name from social provider
- Added `LastName` field for last name from social provider
- Added `ProfilePicture` field for profile picture URL from social provider
- Added `Username` field for username/login from providers (e.g., GitHub login)
- Added `Locale` field for locale from social provider
- Added `RawData` JSONB field to store complete raw JSON response from provider

#### Service Layer Enhancements
- Added `UpdateUser()` method to user repository for updating user profile data
- Enhanced Google login handler to capture: email, verified_email, name, given_name, family_name, picture, locale
- Enhanced Facebook login handler to capture: email, name, first_name, last_name, picture (large), locale
- Enhanced GitHub login handler to capture: email, name, login, avatar_url, bio, location, company
- Implemented smart profile update logic: only update user fields if currently empty when linking social accounts
- Store complete provider response in `RawData` field for all providers

#### API Changes
- Profile endpoint (`GET /profile`) now returns additional fields: name, first_name, last_name, profile_picture, locale
- Social account objects now include all new fields in responses
- No breaking changes - all new fields are optional and nullable

### Changed
- Modified social login data extraction to request extended fields from providers
- Updated Facebook Graph API call to request: `id,name,email,first_name,last_name,picture.type(large),locale`
- Enhanced social account linking to preserve and enrich existing user profile data

### Technical Details
- **Migration Method:** GORM AutoMigrate (automatic on application startup)
- **Database Impact:** Adds 5 columns to `users` table, 8 columns to `social_accounts` table
- **Backward Compatibility:** Fully backward compatible - all new fields are nullable
- **Files Modified:**
  - `pkg/models/user.go` - User model with new profile fields
  - `pkg/models/social_account.go` - Social account model with extended data fields
  - `internal/social/service.go` - Enhanced provider handlers for Google, Facebook, GitHub
  - `internal/user/repository.go` - Added UpdateUser method
  - `docs/migrations/MIGRATION_SOCIAL_LOGIN_DATA.md` - Migration documentation

### Documentation
- Added comprehensive migration documentation in `docs/migrations/MIGRATION_SOCIAL_LOGIN_DATA.md`
- Documents data flow changes, database schema updates, and testing recommendations
- Includes security considerations and rollback plan

---

## [1.0.0] - 2024-01-03

### Features
- User registration and authentication
- Email verification
- Password reset functionality
- Two-factor authentication (TOTP)
- Social login integration (Google, Facebook, GitHub)
- JWT-based authentication (access & refresh tokens)
- Activity logging
- Redis-based session management
- Comprehensive API documentation with Swagger

