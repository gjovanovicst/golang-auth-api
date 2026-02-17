# Migration: Fix Social Account Unique Index for Multi-Tenancy

## Version
`20260120_fix_social_account_unique_index`

## Description
Updates the unique index on the `social_accounts` table to include `app_id`, allowing the same social account (identified by `provider` + `provider_user_id`) to exist in different applications.

## Problem
The original unique index `idx_provider_user_id` was on `(provider, provider_user_id)` only, which prevented the same social user from registering with different applications in a multi-tenant setup.

## Solution
Create a new unique index `idx_provider_user_id_app_id` on `(app_id, provider, provider_user_id)` to scope the uniqueness constraint per application.

## Changes
1. Drop index: `idx_provider_user_id` 
2. Create index: `idx_provider_user_id_app_id` on `(app_id, provider, provider_user_id)`

## Impact
- **Low risk**: Only affects index structure
- **No data loss**: Index recreation only
- **Backwards compatible**: Existing data remains valid

## Execution
```bash
# Apply migration
psql -U postgres -d auth_db -f migrations/20260120_fix_social_account_unique_index.sql

# Or use the migration script
./scripts/migrate.sh
```

## Rollback
```bash
psql -U postgres -d auth_db -f migrations/20260120_fix_social_account_unique_index_rollback.sql
```
