-- Migration: Add system_settings table
-- Date: 2026-02-21
-- Description: Creates the system_settings table for DB-backed configuration management.
--              Settings follow resolution priority: env var > DB value > hardcoded default.

CREATE TABLE IF NOT EXISTS system_settings (
    key VARCHAR(100) PRIMARY KEY,
    value TEXT NOT NULL DEFAULT '',
    category VARCHAR(50) NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Index for fetching settings by category
CREATE INDEX IF NOT EXISTS idx_system_settings_category ON system_settings(category);
