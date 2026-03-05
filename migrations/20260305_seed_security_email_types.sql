-- Migration: Seed security-related email types and default templates
-- Date: 2026-03-05
-- Description: Adds 'new_device_login' and 'suspicious_activity' email types
--              to the email_types registry and seeds global default templates.
--              These types were already implemented in Go code but missing from the DB.

-- 1. Insert the new_device_login email type
INSERT INTO email_types (code, name, description, default_subject, variables, is_system, is_active) VALUES
(
    'new_device_login',
    'New Device Login Notification',
    'Sent when a login is detected from a new device or location not previously seen for this account.',
    'New Login to Your Account',
    '[{"name": "app_name", "description": "Application name", "required": true},
      {"name": "user_email", "description": "User email address", "required": true},
      {"name": "login_ip", "description": "IP address of the login attempt", "required": true},
      {"name": "login_location", "description": "Geographic location of the login (e.g. city, country)", "required": false},
      {"name": "login_device", "description": "Device/browser user-agent of the login", "required": false},
      {"name": "login_time", "description": "Timestamp of the login event", "required": true}]'::jsonb,
    TRUE, TRUE
)
ON CONFLICT (code) DO NOTHING;

-- 2. Insert the suspicious_activity email type
INSERT INTO email_types (code, name, description, default_subject, variables, is_system, is_active) VALUES
(
    'suspicious_activity',
    'Suspicious Activity Alert',
    'Sent when suspicious activity is detected on a user account, such as brute-force attempts or unusual access patterns.',
    'Security Alert: Suspicious Activity on Your Account',
    '[{"name": "app_name", "description": "Application name", "required": true},
      {"name": "user_email", "description": "User email address", "required": true},
      {"name": "login_ip", "description": "IP address of the suspicious activity", "required": true},
      {"name": "login_location", "description": "Geographic location of the activity (e.g. city, country)", "required": false},
      {"name": "login_device", "description": "Device/browser user-agent", "required": false},
      {"name": "login_time", "description": "Timestamp of the event", "required": true},
      {"name": "alert_type", "description": "Type of security alert (e.g. new_device, brute_force)", "required": false},
      {"name": "alert_details", "description": "Detailed description of the security alert", "required": true}]'::jsonb,
    TRUE, TRUE
)
ON CONFLICT (code) DO NOTHING;

-- 3. Insert the default global template for new_device_login
INSERT INTO email_templates (app_id, email_type_id, name, subject, body_html, body_text, template_engine, is_active) VALUES
(
    NULL,
    (SELECT id FROM email_types WHERE code = 'new_device_login'),
    'Default New Device Login Notification',
    'New Login to Your Account',
    '<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>New Login Detected</title>
</head>
<body style="margin:0;padding:0;background-color:#f4f7fa;font-family:-apple-system,BlinkMacSystemFont,''Segoe UI'',Roboto,''Helvetica Neue'',Arial,sans-serif;">
<table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="background-color:#f4f7fa;padding:40px 0;">
<tr><td align="center">
<table role="presentation" width="600" cellspacing="0" cellpadding="0" style="background-color:#ffffff;border-radius:8px;box-shadow:0 2px 8px rgba(0,0,0,0.08);overflow:hidden;">
  <tr><td style="background-color:#3182ce;padding:32px 40px;text-align:center;">
    <h1 style="color:#ffffff;margin:0;font-size:24px;font-weight:600;">{{.AppName}}</h1>
  </td></tr>
  <tr><td style="padding:40px;">
    <h2 style="color:#1a1a2e;margin:0 0 16px;font-size:20px;">New Login Detected</h2>
    <p style="color:#4a5568;font-size:16px;line-height:1.6;margin:0 0 24px;">
      We noticed a new login to your account from a device or location we haven''t seen before:
    </p>
    <table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="background-color:#f7fafc;border-radius:8px;border:1px solid #e2e8f0;margin:0 0 24px;">
      <tr><td style="padding:20px;">
        <table role="presentation" width="100%" cellspacing="0" cellpadding="4">
          <tr>
            <td style="color:#718096;font-size:14px;width:100px;vertical-align:top;padding:4px 8px;">IP Address:</td>
            <td style="color:#1a1a2e;font-size:14px;font-weight:600;padding:4px 8px;">{{.LoginIP}}</td>
          </tr>
          <tr>
            <td style="color:#718096;font-size:14px;vertical-align:top;padding:4px 8px;">Location:</td>
            <td style="color:#1a1a2e;font-size:14px;font-weight:600;padding:4px 8px;">{{.LoginLocation}}</td>
          </tr>
          <tr>
            <td style="color:#718096;font-size:14px;vertical-align:top;padding:4px 8px;">Device:</td>
            <td style="color:#1a1a2e;font-size:14px;padding:4px 8px;">{{.LoginDevice}}</td>
          </tr>
          <tr>
            <td style="color:#718096;font-size:14px;vertical-align:top;padding:4px 8px;">Time:</td>
            <td style="color:#1a1a2e;font-size:14px;font-weight:600;padding:4px 8px;">{{.LoginTime}}</td>
          </tr>
        </table>
      </td></tr>
    </table>
    <p style="color:#4a5568;font-size:16px;line-height:1.6;margin:0 0 16px;">
      If this was you, you can ignore this email. If you don''t recognize this activity, we recommend changing your password immediately.
    </p>
    <p style="color:#a0aec0;font-size:13px;margin:0;">
      This is an automated security notification from {{.AppName}}.
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
    'New Login Detected

We noticed a new login to your {{.AppName}} account:

IP Address: {{.LoginIP}}
Location:   {{.LoginLocation}}
Device:     {{.LoginDevice}}
Time:       {{.LoginTime}}

If this was you, you can ignore this email. If you don''t recognize this activity, we recommend changing your password immediately.

This is an automated security notification from {{.AppName}}.',
    'go_template',
    TRUE
)
ON CONFLICT (email_type_id) WHERE app_id IS NULL DO NOTHING;

-- 4. Insert the default global template for suspicious_activity
INSERT INTO email_templates (app_id, email_type_id, name, subject, body_html, body_text, template_engine, is_active) VALUES
(
    NULL,
    (SELECT id FROM email_types WHERE code = 'suspicious_activity'),
    'Default Suspicious Activity Alert',
    'Security Alert: Suspicious Activity on Your Account',
    '<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Security Alert</title>
</head>
<body style="margin:0;padding:0;background-color:#f4f7fa;font-family:-apple-system,BlinkMacSystemFont,''Segoe UI'',Roboto,''Helvetica Neue'',Arial,sans-serif;">
<table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="background-color:#f4f7fa;padding:40px 0;">
<tr><td align="center">
<table role="presentation" width="600" cellspacing="0" cellpadding="0" style="background-color:#ffffff;border-radius:8px;box-shadow:0 2px 8px rgba(0,0,0,0.08);overflow:hidden;">
  <tr><td style="background-color:#e53e3e;padding:32px 40px;text-align:center;">
    <h1 style="color:#ffffff;margin:0;font-size:24px;font-weight:600;">{{.AppName}}</h1>
  </td></tr>
  <tr><td style="padding:40px;">
    <h2 style="color:#1a1a2e;margin:0 0 16px;font-size:20px;">Security Alert</h2>
    <p style="color:#4a5568;font-size:16px;line-height:1.6;margin:0 0 24px;">
      We detected suspicious activity on your account. Please review the details below:
    </p>
    <table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="background-color:#fff5f5;border-radius:8px;border:1px solid #feb2b2;margin:0 0 24px;">
      <tr><td style="padding:20px;">
        <table role="presentation" width="100%" cellspacing="0" cellpadding="4">
          <tr>
            <td style="color:#718096;font-size:14px;width:100px;vertical-align:top;padding:4px 8px;">Alert:</td>
            <td style="color:#e53e3e;font-size:14px;font-weight:600;padding:4px 8px;">{{.AlertDetails}}</td>
          </tr>
          <tr>
            <td style="color:#718096;font-size:14px;vertical-align:top;padding:4px 8px;">IP Address:</td>
            <td style="color:#1a1a2e;font-size:14px;font-weight:600;padding:4px 8px;">{{.LoginIP}}</td>
          </tr>
          <tr>
            <td style="color:#718096;font-size:14px;vertical-align:top;padding:4px 8px;">Location:</td>
            <td style="color:#1a1a2e;font-size:14px;font-weight:600;padding:4px 8px;">{{.LoginLocation}}</td>
          </tr>
          <tr>
            <td style="color:#718096;font-size:14px;vertical-align:top;padding:4px 8px;">Device:</td>
            <td style="color:#1a1a2e;font-size:14px;padding:4px 8px;">{{.LoginDevice}}</td>
          </tr>
          <tr>
            <td style="color:#718096;font-size:14px;vertical-align:top;padding:4px 8px;">Time:</td>
            <td style="color:#1a1a2e;font-size:14px;font-weight:600;padding:4px 8px;">{{.LoginTime}}</td>
          </tr>
        </table>
      </td></tr>
    </table>
    <p style="color:#e53e3e;font-size:16px;line-height:1.6;margin:0 0 16px;font-weight:600;">
      If you don''t recognize this activity, we strongly recommend:
    </p>
    <ul style="color:#4a5568;font-size:16px;line-height:1.8;margin:0 0 24px;padding-left:24px;">
      <li>Changing your password immediately</li>
      <li>Enabling two-factor authentication if not already active</li>
      <li>Reviewing your recent account activity</li>
    </ul>
    <p style="color:#a0aec0;font-size:13px;margin:0;">
      This is an automated security alert from {{.AppName}}.
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
    'Security Alert: Suspicious Activity on Your Account

We detected suspicious activity on your {{.AppName}} account:

Alert:      {{.AlertDetails}}
IP Address: {{.LoginIP}}
Location:   {{.LoginLocation}}
Device:     {{.LoginDevice}}
Time:       {{.LoginTime}}

If you don''t recognize this activity, we strongly recommend:
- Changing your password immediately
- Enabling two-factor authentication if not already active
- Reviewing your recent account activity

This is an automated security alert from {{.AppName}}.',
    'go_template',
    TRUE
)
ON CONFLICT (email_type_id) WHERE app_id IS NULL DO NOTHING;

-- Register this migration
INSERT INTO schema_migrations (version, name, applied_at, success)
VALUES ('20260305_seed_security_email_types', 'Seed new_device_login and suspicious_activity email types with default templates', NOW(), true)
ON CONFLICT (version) DO NOTHING;
