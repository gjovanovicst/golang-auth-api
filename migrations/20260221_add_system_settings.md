# Migration: Add System Settings Table

**Date**: 2026-02-21  
**File**: `20260221_add_system_settings.sql`  
**Rollback**: `20260221_add_system_settings_rollback.sql`

## Description

Creates the `system_settings` table for DB-backed configuration management. This enables the Admin GUI to view and edit application settings without requiring `.env` file changes and application restarts (for runtime-configurable settings).

## Resolution Priority

Settings follow this resolution order:
1. **Environment variable** (if set and non-empty) — always wins
2. **Database value** (from `system_settings` table) — second priority
3. **Hardcoded default** (in Go code) — fallback

## Schema

| Column | Type | Description |
|--------|------|-------------|
| `key` | `VARCHAR(100)` PK | Setting key, matches env var name (e.g., `ACCESS_TOKEN_EXPIRATION_MINUTES`) |
| `value` | `TEXT` | The setting value as a string |
| `category` | `VARCHAR(50)` | Category for grouping in the GUI (e.g., `general`, `jwt`, `email`) |
| `updated_at` | `TIMESTAMPTZ` | Last modification timestamp |

## Indexes

- `idx_system_settings_category` — For fetching settings by category group

## Application

- Applied automatically by GORM AutoMigrate
- Can also be applied manually: `psql -d auth_db -f migrations/20260221_add_system_settings.sql`

## Rollback

```bash
psql -d auth_db -f migrations/20260221_add_system_settings_rollback.sql
```
