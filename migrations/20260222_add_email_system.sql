-- Migration: Add email system (server configs, types, templates)
-- Date: 2026-02-22
-- Description: Creates tables for the database-driven email system:
--   - email_server_configs: Per-application SMTP server configuration
--   - email_types: Registry of email types (verification, password reset, 2FA code, etc.)
--   - email_templates: Per-application and global default email templates

-- ============================================================================
-- 1. Email Server Configs (per-app SMTP settings)
-- ============================================================================
CREATE TABLE IF NOT EXISTS email_server_configs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    app_id UUID NOT NULL UNIQUE REFERENCES applications(id) ON DELETE CASCADE,
    smtp_host VARCHAR(255) NOT NULL,
    smtp_port INTEGER NOT NULL DEFAULT 587,
    smtp_username VARCHAR(255) DEFAULT '',
    smtp_password TEXT DEFAULT '',
    from_address VARCHAR(255) NOT NULL,
    from_name VARCHAR(100) DEFAULT '',
    use_tls BOOLEAN NOT NULL DEFAULT TRUE,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_email_server_configs_app_id ON email_server_configs(app_id);

-- ============================================================================
-- 2. Email Types (registry of all email categories)
-- ============================================================================
CREATE TABLE IF NOT EXISTS email_types (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code VARCHAR(50) NOT NULL UNIQUE,
    name VARCHAR(100) NOT NULL,
    description TEXT DEFAULT '',
    default_subject VARCHAR(255) DEFAULT '',
    variables JSONB DEFAULT '[]'::jsonb,
    is_system BOOLEAN NOT NULL DEFAULT TRUE,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Seed the 6 system email types
INSERT INTO email_types (code, name, description, default_subject, variables, is_system, is_active) VALUES
(
    'email_verification',
    'Email Verification',
    'Sent when a user registers or changes their email address to verify ownership.',
    'Verify Your Email Address',
    '[{"name": "app_name", "description": "Application name", "required": true},
      {"name": "user_email", "description": "User email address", "required": true},
      {"name": "verification_link", "description": "Email verification URL", "required": true},
      {"name": "verification_token", "description": "Raw verification token", "required": false}]'::jsonb,
    TRUE, TRUE
),
(
    'password_reset',
    'Password Reset',
    'Sent when a user requests a password reset link.',
    'Reset Your Password',
    '[{"name": "app_name", "description": "Application name", "required": true},
      {"name": "user_email", "description": "User email address", "required": true},
      {"name": "reset_link", "description": "Password reset URL", "required": true},
      {"name": "expiration_minutes", "description": "Link expiration time in minutes", "required": false}]'::jsonb,
    TRUE, TRUE
),
(
    'two_fa_code',
    '2FA Verification Code',
    'Sent when a user needs a 2FA verification code via email.',
    'Your Verification Code',
    '[{"name": "app_name", "description": "Application name", "required": true},
      {"name": "user_email", "description": "User email address", "required": true},
      {"name": "code", "description": "6-digit verification code", "required": true},
      {"name": "expiration_minutes", "description": "Code expiration time in minutes", "required": false}]'::jsonb,
    TRUE, TRUE
),
(
    'welcome',
    'Welcome Email',
    'Sent to welcome a user after successful registration and email verification.',
    'Welcome to {{.AppName}}',
    '[{"name": "app_name", "description": "Application name", "required": true},
      {"name": "user_email", "description": "User email address", "required": true},
      {"name": "user_name", "description": "User display name", "required": false}]'::jsonb,
    TRUE, TRUE
),
(
    'account_deactivated',
    'Account Deactivated',
    'Sent when a user account is deactivated by an administrator.',
    'Your Account Has Been Deactivated',
    '[{"name": "app_name", "description": "Application name", "required": true},
      {"name": "user_email", "description": "User email address", "required": true},
      {"name": "user_name", "description": "User display name", "required": false}]'::jsonb,
    TRUE, TRUE
),
(
    'password_changed',
    'Password Changed',
    'Sent as a security notification when a user changes their password.',
    'Your Password Has Been Changed',
    '[{"name": "app_name", "description": "Application name", "required": true},
      {"name": "user_email", "description": "User email address", "required": true},
      {"name": "user_name", "description": "User display name", "required": false},
      {"name": "change_time", "description": "Time of password change", "required": false}]'::jsonb,
    TRUE, TRUE
)
ON CONFLICT (code) DO NOTHING;

-- ============================================================================
-- 3. Email Templates (per-app and global defaults)
-- ============================================================================
CREATE TABLE IF NOT EXISTS email_templates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    app_id UUID REFERENCES applications(id) ON DELETE CASCADE, -- NULL = global default
    email_type_id UUID NOT NULL REFERENCES email_types(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    subject VARCHAR(255) NOT NULL,
    body_html TEXT DEFAULT '',
    body_text TEXT DEFAULT '',
    template_engine VARCHAR(20) NOT NULL DEFAULT 'go_template', -- go_template | placeholder | raw_html
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Unique constraint: one active template per email type per application (or per global default)
CREATE UNIQUE INDEX IF NOT EXISTS idx_app_email_type ON email_templates(app_id, email_type_id)
    WHERE app_id IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_global_email_type ON email_templates(email_type_id)
    WHERE app_id IS NULL;

-- ============================================================================
-- Record Migration
-- ============================================================================
INSERT INTO schema_migrations (version, name, applied_at, success)
VALUES ('20260222_add_email_system', 'Add email system (server configs, types, templates)', NOW(), true)
ON CONFLICT (version) DO NOTHING;
