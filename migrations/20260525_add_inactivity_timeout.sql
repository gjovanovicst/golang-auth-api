-- Migration: 20260525_add_inactivity_timeout
-- Adds inactivity_timeout_minutes column to applications table.
-- 0 = feature disabled (no inactivity enforcement).

ALTER TABLE applications
    ADD COLUMN IF NOT EXISTS inactivity_timeout_minutes INTEGER NOT NULL DEFAULT 0;
