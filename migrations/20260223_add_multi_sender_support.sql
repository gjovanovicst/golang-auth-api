-- Migration: Add multi-sender support
-- Date: 2026-02-23
-- Description: Enables multiple sender addresses and multiple SMTP configs per application.
-- Phase 1: Adds optional from_email/from_name override fields to email_templates
-- Phase 2: Supports multiple SMTP configs per app with name labels and default flagging,
--          and links templates to specific SMTP configs

-- ============================================================================
-- Phase 1: Per-template sender override
-- ============================================================================

-- Add optional sender override fields to email_templates
ALTER TABLE email_templates
    ADD COLUMN IF NOT EXISTS from_email VARCHAR(255) DEFAULT '',
    ADD COLUMN IF NOT EXISTS from_name VARCHAR(255) DEFAULT '';

-- ============================================================================
-- Phase 2: Multiple SMTP configs per application
-- ============================================================================

-- Add name/label and is_default fields to email_server_configs
ALTER TABLE email_server_configs
    ADD COLUMN IF NOT EXISTS name VARCHAR(100) NOT NULL DEFAULT 'Default',
    ADD COLUMN IF NOT EXISTS is_default BOOLEAN NOT NULL DEFAULT TRUE;

-- Drop the old unique constraint on app_id (allows multiple configs per app)
-- The original constraint was: app_id UUID NOT NULL UNIQUE
DROP INDEX IF EXISTS email_server_configs_app_id_key;
-- Also try the GORM-generated index name
DROP INDEX IF EXISTS idx_email_server_configs_app_id;

-- Create a new index (non-unique) for app_id lookups
CREATE INDEX IF NOT EXISTS idx_email_server_configs_app_id ON email_server_configs(app_id);

-- Ensure only one default config per app (partial unique index)
CREATE UNIQUE INDEX IF NOT EXISTS idx_email_server_configs_app_default
    ON email_server_configs(app_id) WHERE is_default = TRUE;

-- Add optional server_config_id foreign key to email_templates
ALTER TABLE email_templates
    ADD COLUMN IF NOT EXISTS server_config_id UUID REFERENCES email_server_configs(id) ON DELETE SET NULL;

-- ============================================================================
-- Record Migration
-- ============================================================================
INSERT INTO schema_migrations (version, name, applied_at, success)
VALUES ('20260223_add_multi_sender_support', 'Add multi-sender support (per-template sender override + multiple SMTP configs per app)', NOW(), true)
ON CONFLICT (version) DO NOTHING;
