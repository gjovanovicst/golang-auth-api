-- Migration: Seed api_key_expiring_soon email type and default template
-- Date: 2026-03-05
-- Description: Adds 'api_key_expiring_soon' email type to the email_types registry
--              and seeds a global default template. This type is used by the background
--              API key expiry notification service to warn admins before keys expire.

-- 1. Insert the email type
INSERT INTO email_types (code, name, description, default_subject, variables, is_system, is_active) VALUES
(
    'api_key_expiring_soon',
    'API Key Expiring Soon',
    'Sent to the system admin when an API key is approaching its expiration date (7-day and 1-day warnings).',
    'API Key Expiring Soon',
    '[{"name": "app_name",          "description": "Application or system name",              "required": true,  "default_value": "Auth API"},
      {"name": "api_key_name",      "description": "Name/label of the expiring API key",      "required": true},
      {"name": "api_key_prefix",    "description": "Key prefix identifier (safe to display)", "required": true},
      {"name": "api_key_type",      "description": "Type of key: admin or app",               "required": true},
      {"name": "api_key_expires_at","description": "Formatted expiry date/time of the key",   "required": true},
      {"name": "days_until_expiry", "description": "Number of days until the key expires",    "required": true}]'::jsonb,
    TRUE, TRUE
)
ON CONFLICT (code) DO NOTHING;

-- 2. Insert the global default template
INSERT INTO email_templates (app_id, email_type_id, name, subject, body_html, body_text, template_engine, is_active) VALUES
(
    NULL,
    (SELECT id FROM email_types WHERE code = 'api_key_expiring_soon'),
    'Default API Key Expiring Soon',
    'API Key ''{{.ApiKeyName}}'' expires in {{.DaysUntilExpiry}} days',
    '<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>API Key Expiring Soon</title>
</head>
<body style="margin:0;padding:0;background-color:#f4f7fa;font-family:-apple-system,BlinkMacSystemFont,''Segoe UI'',Roboto,''Helvetica Neue'',Arial,sans-serif;">
<table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="background-color:#f4f7fa;padding:40px 0;">
<tr><td align="center">
<table role="presentation" width="600" cellspacing="0" cellpadding="0" style="background-color:#ffffff;border-radius:8px;box-shadow:0 2px 8px rgba(0,0,0,0.08);overflow:hidden;">
  <tr><td style="background-color:#4f46e5;padding:32px 40px;text-align:center;">
    <h1 style="color:#ffffff;margin:0;font-size:24px;font-weight:600;">{{.AppName}}</h1>
  </td></tr>
  <tr><td style="padding:40px;">
    <h2 style="color:#1a1a2e;margin:0 0 16px;font-size:20px;">
      <span style="color:#d97706;">&#9888;</span> API Key Expiring Soon
    </h2>
    <p style="color:#4a5568;font-size:16px;line-height:1.6;margin:0 0 24px;">
      An API key in your <strong>{{.AppName}}</strong> account will expire in
      <strong>{{.DaysUntilExpiry}} day{{if ne .DaysUntilExpiry "1"}}s{{end}}</strong>.
      Please rotate this key before it expires to avoid service interruption.
    </p>
    <table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="background-color:#f8fafc;border-radius:6px;padding:20px;margin:0 0 24px;">
      <tr>
        <td style="padding:6px 0;color:#64748b;font-size:14px;width:140px;">Key Name:</td>
        <td style="padding:6px 0;color:#1e293b;font-size:14px;font-weight:600;">{{.ApiKeyName}}</td>
      </tr>
      <tr>
        <td style="padding:6px 0;color:#64748b;font-size:14px;">Key Identifier:</td>
        <td style="padding:6px 0;color:#1e293b;font-size:14px;font-family:monospace;">{{.ApiKeyPrefix}}...</td>
      </tr>
      <tr>
        <td style="padding:6px 0;color:#64748b;font-size:14px;">Key Type:</td>
        <td style="padding:6px 0;color:#1e293b;font-size:14px;text-transform:capitalize;">{{.ApiKeyType}}</td>
      </tr>
      <tr>
        <td style="padding:6px 0;color:#64748b;font-size:14px;">Expires At:</td>
        <td style="padding:6px 0;color:#dc2626;font-size:14px;font-weight:600;">{{.ApiKeyExpiresAt}}</td>
      </tr>
    </table>
    <p style="color:#4a5568;font-size:15px;line-height:1.6;margin:0 0 24px;">
      To rotate this key, log in to the admin panel and generate a new API key, then update your integrations before the expiry date.
    </p>
    <p style="color:#9ca3af;font-size:13px;line-height:1.6;margin:0;">
      This is an automated notification from {{.AppName}}. If you did not expect this email or have already rotated this key, you can safely ignore it.
    </p>
  </td></tr>
  <tr><td style="background-color:#f8fafc;padding:20px 40px;text-align:center;border-top:1px solid #e2e8f0;">
    <p style="color:#94a3b8;font-size:12px;margin:0;">
      &copy; {{.AppName}} &mdash; Automated Security Notification
    </p>
  </td></tr>
</table>
</td></tr>
</table>
</body>
</html>',
    'API Key Expiring Soon

An API key in your {{.AppName}} account will expire in {{.DaysUntilExpiry}} day(s).

Key Name:       {{.ApiKeyName}}
Key Identifier: {{.ApiKeyPrefix}}...
Key Type:       {{.ApiKeyType}}
Expires At:     {{.ApiKeyExpiresAt}}

Please rotate this key before it expires to avoid service interruption.
Log in to the admin panel and generate a new API key, then update your integrations.

This is an automated notification from {{.AppName}}.',
    'go_template',
    TRUE
)
ON CONFLICT (email_type_id) WHERE app_id IS NULL DO NOTHING;

-- Register this migration
INSERT INTO schema_migrations (version, name, applied_at, success)
VALUES ('20260305_seed_api_key_expiring_soon_email_type', 'Seed api_key_expiring_soon email type and default template', NOW(), true)
ON CONFLICT (version) DO NOTHING;
