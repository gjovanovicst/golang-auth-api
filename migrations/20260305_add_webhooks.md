# Migration: Add Webhook System Tables

**Date:** 2026-03-05
**Type:** Schema Change
**Breaking:** No

---

## Overview

Implements the webhook system (feature #11). Applications can register HTTP
endpoint URLs that receive signed `POST` payloads whenever specific auth events
occur (e.g. `user.registered`, `user.login`).

Two tables are added:

- **`webhook_endpoints`** — one registered URL per `(app_id, event_type)` pair.
  Holds the HMAC-SHA256 signing secret (stored only; never returned after creation).
- **`webhook_deliveries`** — full per-attempt delivery history: HTTP status code,
  response body snippet, latency, error message, and next-retry timestamp for
  exponential-backoff retries (up to 5 attempts, max 1-hour interval).

---

## Changes

### Database Schema

**Tables Created:**

- `webhook_endpoints`
- `webhook_deliveries`

**`webhook_endpoints` columns:**

| Column | Type | Notes |
|---|---|---|
| `id` | `UUID` PK | `gen_random_uuid()` default |
| `app_id` | `UUID NOT NULL` | FK → `applications(id)` ON DELETE CASCADE |
| `event_type` | `VARCHAR(64) NOT NULL` | CHECK constraint — 8 valid values |
| `url` | `TEXT NOT NULL` | Target endpoint URL |
| `secret` | `TEXT NOT NULL` | HMAC-SHA256 key; shown only once at creation |
| `is_active` | `BOOLEAN NOT NULL DEFAULT TRUE` | Soft-disable without deleting |
| `created_at` | `TIMESTAMPTZ NOT NULL DEFAULT NOW()` | |
| `updated_at` | `TIMESTAMPTZ NOT NULL DEFAULT NOW()` | |
| `deleted_at` | `TIMESTAMPTZ` | Soft-delete (GORM) |

**`webhook_endpoints` indexes & constraints:**

| Name | Definition | Purpose |
|---|---|---|
| `pk_webhook_endpoints` | PRIMARY KEY (`id`) | |
| `chk_webhook_event_type` | CHECK (`event_type` IN …) | Enforce 8 valid event types |
| `idx_webhook_app_event` | UNIQUE (`app_id`, `event_type`) WHERE `deleted_at IS NULL` | One endpoint per app/event |
| `idx_webhook_endpoints_app_id` | (`app_id`) | Filter by app |
| `idx_webhook_endpoints_deleted_at` | (`deleted_at`) | Soft-delete scans |

**`webhook_deliveries` columns:**

| Column | Type | Notes |
|---|---|---|
| `id` | `UUID` PK | `gen_random_uuid()` default |
| `endpoint_id` | `UUID NOT NULL` | FK → `webhook_endpoints(id)` ON DELETE CASCADE |
| `app_id` | `UUID NOT NULL` | Denormalised for fast per-app queries |
| `event_type` | `VARCHAR(64) NOT NULL` | Copied from endpoint at dispatch time |
| `payload` | `TEXT NOT NULL` | Full JSON payload sent |
| `attempt` | `INT NOT NULL DEFAULT 1` | 1-based attempt counter |
| `status_code` | `INT` | HTTP response code; 0 = no response received |
| `response_body` | `TEXT` | First ~1 KB of response body |
| `latency_ms` | `BIGINT` | Round-trip time in milliseconds |
| `success` | `BOOLEAN NOT NULL DEFAULT FALSE` | `true` = 2xx response |
| `error_message` | `TEXT` | Network / timeout error text |
| `next_retry_at` | `TIMESTAMPTZ` | `NULL` = no further retries scheduled |
| `created_at` | `TIMESTAMPTZ NOT NULL DEFAULT NOW()` | |

**`webhook_deliveries` indexes:**

| Name | Definition | Purpose |
|---|---|---|
| `pk_webhook_deliveries` | PRIMARY KEY (`id`) | |
| `idx_webhook_deliveries_endpoint_id` | (`endpoint_id`) | Delivery history per endpoint |
| `idx_webhook_deliveries_app_id` | (`app_id`) | Per-app delivery queries |
| `idx_webhook_deliveries_event_type` | (`event_type`) | Filter by event |
| `idx_webhook_deliveries_success` | (`success`) | Filter failures |
| `idx_webhook_deliveries_next_retry_at` | (`next_retry_at`) WHERE NOT NULL | Retry worker polling |

---

## Migration Files

**Forward:** `migrations/20260305_add_webhooks.sql`
**Rollback:** `migrations/20260305_add_webhooks_rollback.sql`

---

## Impact Assessment

### Breaking Changes

**No.** Purely additive — new tables only. Existing tables and API contracts are unchanged.

### Performance Impact

- Migration time: < 1 second on an empty or small DB; near-instant DDL on PostgreSQL.
- Application impact: none — no downtime required; tables are created before the app reads them.
- Storage: minimal until webhooks are registered and events fire.

### Compatibility

- **Backward Compatible:** Yes
- **Forward Compatible:** Yes
- **Minimum App Version:** requires the webhook service code introduced alongside this migration
- **Requires Configuration Changes:** No

---

## Applying the Migration

**Development (automatic via GORM AutoMigrate on startup):**
```bash
make dev
```

**Manual apply:**
```bash
psql -U $DB_USER -d $DB_NAME -f migrations/20260305_add_webhooks.sql
```

**Rollback:**
```bash
psql -U $DB_USER -d $DB_NAME -f migrations/20260305_add_webhooks_rollback.sql
```

---

## Verification

```sql
-- Confirm tables exist
\d webhook_endpoints
\d webhook_deliveries

-- Confirm unique index (partial — excludes soft-deleted rows)
\di webhook_endpoints*

-- Confirm event_type constraint
INSERT INTO webhook_endpoints (app_id, event_type, url, secret)
VALUES (gen_random_uuid(), 'invalid.event', 'https://example.com', 'x');
-- Expected: ERROR: new row for relation "webhook_endpoints" violates check constraint

-- Confirm cascade: deleting an app removes its endpoints and deliveries
-- (test in a dev environment with a disposable app row)

-- Confirm schema_migrations row
SELECT version, name, applied_at, success
FROM schema_migrations
WHERE version = '20260305_add_webhooks';
```

---

## Notes

- The `secret` column is write-only from the application's perspective: the raw
  `whsec_<hex>` value is generated in-process, shown once in the API/GUI response,
  and then only the stored HMAC key is used for signing — it is never returned again.
- The `idx_webhook_app_event` unique index is a **partial index** (`WHERE deleted_at IS NULL`)
  so that soft-deleted endpoints do not block re-registration of the same
  `(app_id, event_type)` pair.
- `webhook_deliveries.app_id` is denormalised (copied from the endpoint at dispatch
  time) to allow efficient per-app delivery queries without a join.
