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

## WebAuthn / Passkeys

**Impact:** Non-breaking (new feature, opt-in)

- New database table: `webauthn_credentials` (created automatically by GORM AutoMigrate)
- New fields on `applications` table: `passkey_2fa_enabled`, `passkey_login_enabled`
- New environment variables: `WEBAUTHN_RP_ID`, `WEBAUTHN_RP_NAME`, `WEBAUTHN_RP_ORIGINS`
- New API endpoints under `/passkey/*` and `/2fa/passkey/*`

**Action required:** Set the `WEBAUTHN_*` environment variables in `.env` if you want to enable passkey support. No migration script needed -- GORM handles the schema automatically.

---

## Role-Based Access Control (RBAC)

**Impact:** Potentially breaking for JWT consumers

- New database tables: `roles`, `permissions`, `user_roles` (SQL migration required)
- JWT tokens now include a `roles` claim (array of role names)
- Default system roles `admin` and `member` are seeded automatically
- All existing users are assigned the `member` role via backfill migration

**Action required:**
1. Run `make migrate-up` to apply RBAC migrations (`20260301_add_rbac.sql`, `20260301_seed_rbac_defaults.sql`, `20260302_backfill_member_role.sql`)
2. If your application parses JWT claims, update it to handle the new `roles` array

---

## Session Management + Auth Middleware Hardening

**Impact:** Potentially breaking for long-lived sessions

- Auth middleware now validates session existence in Redis on every authenticated request
- If a session is revoked or expired in Redis, the request is rejected even if the JWT is still valid
- New API endpoints: `GET /sessions`, `DELETE /sessions/:id`, `DELETE /sessions`

**Action required:** No migration needed. Be aware that revoking a session now immediately invalidates all requests using that session's token, rather than waiting for JWT expiry.

---

## Magic Link Login

**Impact:** Non-breaking (new feature, opt-in)

- New application setting: `magic_link_enabled` (default: disabled)
- New admin account field: `magic_link_enabled`
- New API endpoints: `POST /magic-link/request`, `POST /magic-link/verify`
- SQL migrations required for the magic link email type and settings

**Action required:** Run `make migrate-up` to apply magic link migrations (`20260303_add_admin_magic_link.sql`, `20260303_add_magic_link_settings.sql`, `20260303_seed_magic_link_email_type.sql`). Enable per-application via Admin API.

---

## Social Account Linking

**Impact:** Non-breaking (new feature)

- Users can now link/unlink additional social accounts to their existing profile
- New API endpoints: `GET /profile/social-accounts`, `DELETE /profile/social-accounts`, `/auth/{provider}/link`, `/auth/{provider}/link/callback`

**Action required:** None. Feature is available immediately after upgrade.

---

## Application Model Changes

**Impact:** Non-breaking (additive)

New fields added to the `applications` table (managed by GORM AutoMigrate):
- `passkey_2fa_enabled` -- Enable passkey as a 2FA method
- `passkey_login_enabled` -- Enable passwordless passkey login
- `magic_link_enabled` -- Enable magic link email login
- `email_2fa_enabled` -- Enable email-based 2FA
- `two_fa_methods` -- Configured 2FA methods

**Action required:** None. All fields have safe defaults and are added automatically on startup.

---

## API Key Empty Scope: Deny-by-Default (alpha.7)

**Impact:** Breaking for existing DB-backed API keys with no scopes configured

**Vulnerability:** CWE-269 — Improper Privilege Management. A DB-backed API key issued without any scopes was previously treated as fully permissive by `HasScope()`, allowing privilege escalation to admin-level access.

**Fix:** `internal/middleware/scope.go` — `HasScope()` now **denies by default** when the granted scope list for a DB-backed key is empty. The static `ADMIN_API_KEY` environment variable is unaffected (always permissive).

**Affected versions:** `1.0.0-alpha.6` and earlier, for any DB-backed API key created without scopes.

**Action required:** Review all DB-backed API keys in the Admin GUI (`/gui/api-keys`). Any key that was created without scopes to obtain full access must be updated:
- To grant unrestricted access, add the `"*"` scope to the key.
- To follow least privilege, add only the specific scopes the key needs.

API keys authenticated via the static `ADMIN_API_KEY` environment variable are not affected.

Credit: **tinokyo** ([@Tinocio](https://github.com/Tinocio)) — responsible disclosure, root cause analysis, and proof of concept.

---

## RBAC Member Role: Settings Permissions (alpha.7)

**Impact:** Non-breaking (additive migration — fixes missing permissions)

The system `member` role was missing `settings:read` and `settings:write` permissions. This caused 403 errors for all regular users on 2FA self-service endpoints (TOTP setup, email 2FA, SMS 2FA, backup email, passkeys, trusted devices, and phone management).

**Action required:** Run `make migrate-up` to apply `20260317_add_settings_permissions_to_member.sql`. This grants the missing permissions to the `member` system role in every existing application.

```bash
make migrate-up
```

No code changes are required. The migration is safe to apply to existing data.

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
