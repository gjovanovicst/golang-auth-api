# Migration: Add API Keys Table

**Date**: 2026-02-20  
**File**: `20260220_add_api_keys.sql`  
**Rollback**: `20260220_add_api_keys_rollback.sql`

## Description

Creates the `api_keys` table for managing API keys. Supports two key types:

- **admin**: Authenticates to `/admin/*` JSON API routes (replaces/supplements the static `ADMIN_API_KEY` env var)
- **app**: Authenticates to per-application routes alongside the `X-App-ID` header

## Schema

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| `id` | UUID | No | `gen_random_uuid()` | Primary key |
| `key_type` | VARCHAR(10) | No | - | `"admin"` or `"app"` |
| `name` | VARCHAR(255) | No | - | Human-readable label |
| `description` | TEXT | Yes | `''` | Optional purpose description |
| `key_hash` | VARCHAR(64) | No | - | SHA-256 hash of the raw key (unique) |
| `key_prefix` | VARCHAR(16) | No | - | First 8 chars for display |
| `key_suffix` | VARCHAR(4) | No | - | Last 4 chars for identification |
| `app_id` | UUID | Yes | - | FK to `applications(id)`, required when `key_type = "app"` |
| `expires_at` | TIMESTAMPTZ | Yes | - | Optional expiration timestamp |
| `last_used_at` | TIMESTAMPTZ | Yes | - | Updated on each successful authentication |
| `is_revoked` | BOOLEAN | No | `false` | Revocation flag |
| `created_at` | TIMESTAMPTZ | No | `NOW()` | Creation timestamp |
| `updated_at` | TIMESTAMPTZ | No | `NOW()` | Last update timestamp |

## Security Notes

- Raw keys are never stored. Only SHA-256 hashes are persisted.
- `key_prefix` and `key_suffix` allow visual identification without exposing the full key.
- The `idx_api_keys_active_lookup` partial index optimizes middleware lookups for active keys.
- `app_id` has a CASCADE delete constraint — deleting an application removes its API keys.

## Rollback

The rollback script drops all indexes and the `api_keys` table.
