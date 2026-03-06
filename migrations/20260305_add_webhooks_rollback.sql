-- Rollback: Drop webhook_endpoints and webhook_deliveries tables
-- Reverses: 20260305_add_webhooks.sql

BEGIN;

-- Drop deliveries first (has FK → webhook_endpoints)
DROP TABLE IF EXISTS webhook_deliveries;

-- Drop endpoints (has FK → applications)
DROP TABLE IF EXISTS webhook_endpoints;

DELETE FROM schema_migrations WHERE version = '20260305_add_webhooks';

COMMIT;
