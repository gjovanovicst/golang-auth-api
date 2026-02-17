# Add Multi-Tenancy

## Changes
- Create `tenants` table
- Create `applications` table
- Create `oauth_provider_configs` table
- Add `app_id` to `users`, `social_accounts`, `activity_logs`
- Migrate existing data to default tenant/app

## Rollback
- Drop tables and columns
