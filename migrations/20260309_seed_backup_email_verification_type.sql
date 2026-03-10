-- Migration: Seed backup_email_verification email type and default template
-- Date: 2026-03-09
-- Description: Adds the 'backup_email_verification' email type to the email_types registry
--              and seeds a global default template for backup email address verification emails.
--              This type was already implemented in Go code (internal/email/defaults.go) but
--              was missing from the database, preventing admin customisation via the GUI.

-- 1. Insert the backup_email_verification email type
INSERT INTO email_types (code, name, description, default_subject, variables, is_system, is_active) VALUES
(
    'backup_email_verification',
    'Backup Email Verification',
    'Sent when a user registers a backup email address for 2FA recovery. Contains a verification link the user must click to confirm ownership of the backup address.',
    'Verify Your Backup Email Address',
    '[{"name": "app_name",            "description": "Application name",                                              "required": true},
      {"name": "backup_email",        "description": "The backup email address being verified",                       "required": true},
      {"name": "verification_link",   "description": "Full URL the user must click to verify the backup address",     "required": true},
      {"name": "expiration_minutes",  "description": "Number of minutes before the verification link expires",        "required": false}]'::jsonb,
    TRUE, TRUE
)
ON CONFLICT (code) DO NOTHING;

-- 2. Insert the default global template
INSERT INTO email_templates (app_id, email_type_id, name, subject, body_html, body_text, template_engine, is_active) VALUES
(
    NULL,
    (SELECT id FROM email_types WHERE code = 'backup_email_verification'),
    'Default Backup Email Verification',
    'Verify Your Backup Email Address',
    '<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Verify Your Backup Email</title>
</head>
<body style="margin:0;padding:0;background-color:#f4f7fa;font-family:-apple-system,BlinkMacSystemFont,''Segoe UI'',Roboto,''Helvetica Neue'',Arial,sans-serif;">
<table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="background-color:#f4f7fa;padding:40px 0;">
<tr><td align="center">
<table role="presentation" width="600" cellspacing="0" cellpadding="0" style="background-color:#ffffff;border-radius:8px;box-shadow:0 2px 8px rgba(0,0,0,0.08);overflow:hidden;">
  <tr><td style="background-color:#4f46e5;padding:32px 40px;text-align:center;">
    <h1 style="color:#ffffff;margin:0;font-size:24px;font-weight:600;">{{.AppName}}</h1>
  </td></tr>
  <tr><td style="padding:40px;">
    <h2 style="color:#1a1a2e;margin:0 0 16px;font-size:20px;">Verify Your Backup Email Address</h2>
    <p style="color:#4a5568;font-size:16px;line-height:1.6;margin:0 0 24px;">
      You requested to register <strong>{{.BackupEmail}}</strong> as a backup email address for account recovery.
      Please click the button below to verify this email address.
    </p>
    <table role="presentation" cellspacing="0" cellpadding="0" style="margin:0 auto 24px;">
    <tr><td style="background-color:#4f46e5;border-radius:6px;">
      <a href="{{.VerificationLink}}" style="display:inline-block;padding:14px 32px;color:#ffffff;text-decoration:none;font-size:16px;font-weight:600;">Verify Backup Email</a>
    </td></tr>
    </table>
    <p style="color:#718096;font-size:14px;line-height:1.5;margin:0 0 8px;">
      If the button doesn''t work, copy and paste this link into your browser:
    </p>
    <p style="color:#4f46e5;font-size:14px;word-break:break-all;margin:0 0 24px;">{{.VerificationLink}}</p>
    <p style="color:#e53e3e;font-size:14px;line-height:1.5;margin:0 0 16px;">
      This link will expire in {{.ExpirationMinutes}} minutes.
    </p>
    <p style="color:#a0aec0;font-size:13px;margin:0;">
      If you did not request this, you can safely ignore this email. Your primary account will not be affected.
    </p>
  </td></tr>
  <tr><td style="background-color:#f8fafc;padding:24px 40px;text-align:center;border-top:1px solid #e2e8f0;">
    <p style="color:#a0aec0;font-size:12px;margin:0;">This email was sent by {{.AppName}}. Please do not reply to this email.</p>
  </td></tr>
</table>
</td></tr>
</table>
</body>
</html>',
    'Verify Your Backup Email Address

You requested to register {{.BackupEmail}} as a backup email address for your {{.AppName}} account recovery.

Please verify this email address by clicking the link below:
{{.VerificationLink}}

This link will expire in {{.ExpirationMinutes}} minutes.

If you did not request this, you can safely ignore this email.',
    'go_template',
    TRUE
)
ON CONFLICT (email_type_id) WHERE app_id IS NULL DO NOTHING;

-- Register this migration
INSERT INTO schema_migrations (version, name, applied_at, success)
VALUES ('20260309_seed_backup_email_verification_type', 'Seed backup_email_verification email type and default template', NOW(), true)
ON CONFLICT (version) DO NOTHING;
