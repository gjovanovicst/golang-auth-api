-- Migration: Seed magic link email type and default template
-- Date: 2026-03-03
-- Description: Adds the 'magic_link' email type to the email_types registry
--              and seeds a global default template for magic link login emails.

-- 1. Insert the magic_link email type
INSERT INTO email_types (code, name, description, default_subject, variables, is_system, is_active) VALUES
(
    'magic_link',
    'Magic Link Login',
    'Sent when a user requests a passwordless login via email magic link.',
    'Sign In to Your Account',
    '[{"name": "app_name", "description": "Application name", "required": true},
      {"name": "user_email", "description": "User email address", "required": true},
      {"name": "magic_link", "description": "Magic link login URL", "required": true},
      {"name": "expiration_minutes", "description": "Link expiration time in minutes", "required": false}]'::jsonb,
    TRUE, TRUE
)
ON CONFLICT (code) DO NOTHING;

-- 2. Insert the default global template
INSERT INTO email_templates (app_id, email_type_id, name, subject, body_html, body_text, template_engine, is_active) VALUES
(
    NULL,
    (SELECT id FROM email_types WHERE code = 'magic_link'),
    'Default Magic Link Login',
    'Sign In to Your Account',
    '<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Sign In to Your Account</title>
</head>
<body style="margin:0;padding:0;background-color:#f4f7fa;font-family:-apple-system,BlinkMacSystemFont,''Segoe UI'',Roboto,''Helvetica Neue'',Arial,sans-serif;">
<table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="background-color:#f4f7fa;padding:40px 0;">
<tr><td align="center">
<table role="presentation" width="600" cellspacing="0" cellpadding="0" style="background-color:#ffffff;border-radius:8px;box-shadow:0 2px 8px rgba(0,0,0,0.08);overflow:hidden;">
  <tr><td style="background-color:#4f46e5;padding:32px 40px;text-align:center;">
    <h1 style="color:#ffffff;margin:0;font-size:24px;font-weight:600;">{{.AppName}}</h1>
  </td></tr>
  <tr><td style="padding:40px;">
    <h2 style="color:#1a1a2e;margin:0 0 16px;font-size:20px;">Sign In to Your Account</h2>
    <p style="color:#4a5568;font-size:16px;line-height:1.6;margin:0 0 24px;">
      We received a request to sign in to your account. Click the button below to log in instantly — no password needed.
    </p>
    <table role="presentation" cellspacing="0" cellpadding="0" style="margin:0 auto 24px;">
    <tr><td style="background-color:#4f46e5;border-radius:6px;">
      <a href="{{.MagicLink}}" style="display:inline-block;padding:14px 32px;color:#ffffff;text-decoration:none;font-size:16px;font-weight:600;">Sign In Now</a>
    </td></tr>
    </table>
    <p style="color:#718096;font-size:14px;line-height:1.5;margin:0 0 8px;">
      If the button doesn''t work, copy and paste this link into your browser:
    </p>
    <p style="color:#4f46e5;font-size:14px;word-break:break-all;margin:0 0 24px;">{{.MagicLink}}</p>
    <p style="color:#e53e3e;font-size:14px;line-height:1.5;margin:0 0 16px;">
      This link will expire in {{.ExpirationMinutes}} minutes and can only be used once.
    </p>
    <p style="color:#a0aec0;font-size:13px;margin:0;">
      If you didn''t request this link, you can safely ignore this email. No one can access your account without clicking the link above.
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
    'Sign In to Your Account

We received a request to sign in to your {{.AppName}} account.

Click the link below to log in instantly:
{{.MagicLink}}

This link will expire in {{.ExpirationMinutes}} minutes and can only be used once.

If you didn''t request this link, you can safely ignore this email.',
    'go_template',
    TRUE
)
ON CONFLICT (email_type_id) WHERE app_id IS NULL DO NOTHING;

-- Register this migration
INSERT INTO schema_migrations (version, name, applied_at, success)
VALUES ('20260303_seed_magic_link_email_type', 'Seed magic link email type and default template', NOW(), true)
ON CONFLICT (version) DO NOTHING;
