-- Migration: Seed Centrora invitation email types and default templates
-- Date: 2026-05-14
-- Description: Adds three email type codes used by the Centrora outbox mailer:
--   centrora_invitation         – org-level invitation
--   centrora_project_invitation – project-level invitation
--   centrora_env_invitation     – environment-level invitation
-- Variables common to all three: inviter_name, organization_name, role, accept_url, expires_at
-- Additional variables: project_name (project + env types), env_name (env type)

-- -------------------------------------------------------------------------
-- 1. Email types
-- -------------------------------------------------------------------------

INSERT INTO email_types (code, name, description, default_subject, variables, is_system, is_active) VALUES
(
    'centrora_invitation',
    'Centrora Organization Invitation',
    'Sent when a user is invited to join an organization in Centrora.',
    'You have been invited to join {{organization_name}}',
    '[{"name": "inviter_name",      "description": "Name or email of the person who sent the invite", "required": true},
      {"name": "organization_name", "description": "Name of the organization",                        "required": true},
      {"name": "role",              "description": "Role the invitee will receive (owner/admin/member/viewer)", "required": true},
      {"name": "accept_url",        "description": "URL the invitee clicks to accept the invitation", "required": true},
      {"name": "expires_at",        "description": "Human-readable expiry date/time of the invite",   "required": false}]'::jsonb,
    TRUE, TRUE
)
ON CONFLICT (code) DO NOTHING;

INSERT INTO email_types (code, name, description, default_subject, variables, is_system, is_active) VALUES
(
    'centrora_project_invitation',
    'Centrora Project Invitation',
    'Sent when a user is invited to a project inside an organization in Centrora.',
    'You have been invited to the {{project_name}} project',
    '[{"name": "inviter_name",      "description": "Name or email of the person who sent the invite", "required": true},
      {"name": "organization_name", "description": "Name of the organization",                        "required": true},
      {"name": "project_name",      "description": "Name of the project",                             "required": true},
      {"name": "role",              "description": "Role the invitee will receive",                   "required": true},
      {"name": "accept_url",        "description": "URL the invitee clicks to accept the invitation", "required": true},
      {"name": "expires_at",        "description": "Human-readable expiry date/time of the invite",   "required": false}]'::jsonb,
    TRUE, TRUE
)
ON CONFLICT (code) DO NOTHING;

INSERT INTO email_types (code, name, description, default_subject, variables, is_system, is_active) VALUES
(
    'centrora_env_invitation',
    'Centrora Environment Invitation',
    'Sent when a user is invited to a specific environment inside a project in Centrora.',
    'You have been invited to the {{env_name}} environment',
    '[{"name": "inviter_name",      "description": "Name or email of the person who sent the invite", "required": true},
      {"name": "organization_name", "description": "Name of the organization",                        "required": true},
      {"name": "project_name",      "description": "Name of the project",                             "required": true},
      {"name": "env_name",          "description": "Name of the environment",                         "required": true},
      {"name": "role",              "description": "Role the invitee will receive",                   "required": true},
      {"name": "accept_url",        "description": "URL the invitee clicks to accept the invitation", "required": true},
      {"name": "expires_at",        "description": "Human-readable expiry date/time of the invite",   "required": false}]'::jsonb,
    TRUE, TRUE
)
ON CONFLICT (code) DO NOTHING;

-- -------------------------------------------------------------------------
-- 2. Default global templates
-- -------------------------------------------------------------------------

-- centrora_invitation
INSERT INTO email_templates (app_id, email_type_id, name, subject, body_html, body_text, template_engine, is_active) VALUES
(
    NULL,
    (SELECT id FROM email_types WHERE code = 'centrora_invitation'),
    'Default Centrora Organization Invitation',
    'You have been invited to join {{.OrganizationName}}',
    '<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Organization Invitation</title>
</head>
<body style="margin:0;padding:0;background-color:#f4f7fa;font-family:-apple-system,BlinkMacSystemFont,''Segoe UI'',Roboto,''Helvetica Neue'',Arial,sans-serif;">
<table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="background-color:#f4f7fa;padding:40px 0;">
<tr><td align="center">
<table role="presentation" width="600" cellspacing="0" cellpadding="0" style="background-color:#ffffff;border-radius:8px;box-shadow:0 2px 8px rgba(0,0,0,0.08);overflow:hidden;">
  <tr><td style="background-color:#4f46e5;padding:32px 40px;text-align:center;">
    <h1 style="color:#ffffff;margin:0;font-size:24px;font-weight:600;">Centrora</h1>
  </td></tr>
  <tr><td style="padding:40px;">
    <h2 style="color:#1a1a2e;margin:0 0 16px;font-size:20px;">You''re Invited!</h2>
    <p style="color:#4a5568;font-size:16px;line-height:1.6;margin:0 0 24px;">
      <strong>{{.InviterName}}</strong> has invited you to join the <strong>{{.OrganizationName}}</strong> organization with the role <strong>{{.Role}}</strong>.
    </p>
    <table role="presentation" cellspacing="0" cellpadding="0" style="margin:0 auto 24px;">
    <tr><td style="background-color:#4f46e5;border-radius:6px;">
      <a href="{{.AcceptURL}}" style="display:inline-block;padding:14px 32px;color:#ffffff;text-decoration:none;font-size:16px;font-weight:600;">Accept Invitation</a>
    </td></tr>
    </table>
    <p style="color:#718096;font-size:14px;line-height:1.5;margin:0 0 8px;">
      If the button doesn''t work, copy and paste this link into your browser:
    </p>
    <p style="color:#4f46e5;font-size:14px;word-break:break-all;margin:0 0 24px;">{{.AcceptURL}}</p>
    {{if .ExpiresAt}}<p style="color:#e53e3e;font-size:14px;line-height:1.5;margin:0 0 16px;">This invitation expires on {{.ExpiresAt}}.</p>{{end}}
    <p style="color:#a0aec0;font-size:13px;margin:0;">
      If you were not expecting this invitation you can safely ignore this email.
    </p>
  </td></tr>
  <tr><td style="background-color:#f8fafc;padding:24px 40px;text-align:center;border-top:1px solid #e2e8f0;">
    <p style="color:#a0aec0;font-size:12px;margin:0;">This email was sent by Centrora. Please do not reply to this email.</p>
  </td></tr>
</table>
</td></tr>
</table>
</body>
</html>',
    'You''re Invited to {{.OrganizationName}}

{{.InviterName}} has invited you to join the {{.OrganizationName}} organization with the role {{.Role}}.

Accept the invitation by clicking the link below:
{{.AcceptURL}}
{{if .ExpiresAt}}
This invitation expires on {{.ExpiresAt}}.
{{end}}
If you were not expecting this invitation you can safely ignore this email.',
    'go_template',
    TRUE
)
ON CONFLICT (email_type_id) WHERE app_id IS NULL DO NOTHING;

-- centrora_project_invitation
INSERT INTO email_templates (app_id, email_type_id, name, subject, body_html, body_text, template_engine, is_active) VALUES
(
    NULL,
    (SELECT id FROM email_types WHERE code = 'centrora_project_invitation'),
    'Default Centrora Project Invitation',
    'You have been invited to the {{.ProjectName}} project',
    '<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Project Invitation</title>
</head>
<body style="margin:0;padding:0;background-color:#f4f7fa;font-family:-apple-system,BlinkMacSystemFont,''Segoe UI'',Roboto,''Helvetica Neue'',Arial,sans-serif;">
<table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="background-color:#f4f7fa;padding:40px 0;">
<tr><td align="center">
<table role="presentation" width="600" cellspacing="0" cellpadding="0" style="background-color:#ffffff;border-radius:8px;box-shadow:0 2px 8px rgba(0,0,0,0.08);overflow:hidden;">
  <tr><td style="background-color:#4f46e5;padding:32px 40px;text-align:center;">
    <h1 style="color:#ffffff;margin:0;font-size:24px;font-weight:600;">Centrora</h1>
  </td></tr>
  <tr><td style="padding:40px;">
    <h2 style="color:#1a1a2e;margin:0 0 16px;font-size:20px;">Project Invitation</h2>
    <p style="color:#4a5568;font-size:16px;line-height:1.6;margin:0 0 24px;">
      <strong>{{.InviterName}}</strong> has invited you to the <strong>{{.ProjectName}}</strong> project inside <strong>{{.OrganizationName}}</strong> with the role <strong>{{.Role}}</strong>.
    </p>
    <table role="presentation" cellspacing="0" cellpadding="0" style="margin:0 auto 24px;">
    <tr><td style="background-color:#4f46e5;border-radius:6px;">
      <a href="{{.AcceptURL}}" style="display:inline-block;padding:14px 32px;color:#ffffff;text-decoration:none;font-size:16px;font-weight:600;">Accept Invitation</a>
    </td></tr>
    </table>
    <p style="color:#718096;font-size:14px;line-height:1.5;margin:0 0 8px;">
      If the button doesn''t work, copy and paste this link into your browser:
    </p>
    <p style="color:#4f46e5;font-size:14px;word-break:break-all;margin:0 0 24px;">{{.AcceptURL}}</p>
    {{if .ExpiresAt}}<p style="color:#e53e3e;font-size:14px;line-height:1.5;margin:0 0 16px;">This invitation expires on {{.ExpiresAt}}.</p>{{end}}
    <p style="color:#a0aec0;font-size:13px;margin:0;">
      If you were not expecting this invitation you can safely ignore this email.
    </p>
  </td></tr>
  <tr><td style="background-color:#f8fafc;padding:24px 40px;text-align:center;border-top:1px solid #e2e8f0;">
    <p style="color:#a0aec0;font-size:12px;margin:0;">This email was sent by Centrora. Please do not reply to this email.</p>
  </td></tr>
</table>
</td></tr>
</table>
</body>
</html>',
    'Project Invitation: {{.ProjectName}}

{{.InviterName}} has invited you to the {{.ProjectName}} project inside {{.OrganizationName}} with the role {{.Role}}.

Accept the invitation by clicking the link below:
{{.AcceptURL}}
{{if .ExpiresAt}}
This invitation expires on {{.ExpiresAt}}.
{{end}}
If you were not expecting this invitation you can safely ignore this email.',
    'go_template',
    TRUE
)
ON CONFLICT (email_type_id) WHERE app_id IS NULL DO NOTHING;

-- centrora_env_invitation
INSERT INTO email_templates (app_id, email_type_id, name, subject, body_html, body_text, template_engine, is_active) VALUES
(
    NULL,
    (SELECT id FROM email_types WHERE code = 'centrora_env_invitation'),
    'Default Centrora Environment Invitation',
    'You have been invited to the {{.EnvName}} environment',
    '<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Environment Invitation</title>
</head>
<body style="margin:0;padding:0;background-color:#f4f7fa;font-family:-apple-system,BlinkMacSystemFont,''Segoe UI'',Roboto,''Helvetica Neue'',Arial,sans-serif;">
<table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="background-color:#f4f7fa;padding:40px 0;">
<tr><td align="center">
<table role="presentation" width="600" cellspacing="0" cellpadding="0" style="background-color:#ffffff;border-radius:8px;box-shadow:0 2px 8px rgba(0,0,0,0.08);overflow:hidden;">
  <tr><td style="background-color:#4f46e5;padding:32px 40px;text-align:center;">
    <h1 style="color:#ffffff;margin:0;font-size:24px;font-weight:600;">Centrora</h1>
  </td></tr>
  <tr><td style="padding:40px;">
    <h2 style="color:#1a1a2e;margin:0 0 16px;font-size:20px;">Environment Invitation</h2>
    <p style="color:#4a5568;font-size:16px;line-height:1.6;margin:0 0 24px;">
      <strong>{{.InviterName}}</strong> has invited you to the <strong>{{.EnvName}}</strong> environment in the <strong>{{.ProjectName}}</strong> project (<strong>{{.OrganizationName}}</strong>) with the role <strong>{{.Role}}</strong>.
    </p>
    <table role="presentation" cellspacing="0" cellpadding="0" style="margin:0 auto 24px;">
    <tr><td style="background-color:#4f46e5;border-radius:6px;">
      <a href="{{.AcceptURL}}" style="display:inline-block;padding:14px 32px;color:#ffffff;text-decoration:none;font-size:16px;font-weight:600;">Accept Invitation</a>
    </td></tr>
    </table>
    <p style="color:#718096;font-size:14px;line-height:1.5;margin:0 0 8px;">
      If the button doesn''t work, copy and paste this link into your browser:
    </p>
    <p style="color:#4f46e5;font-size:14px;word-break:break-all;margin:0 0 24px;">{{.AcceptURL}}</p>
    {{if .ExpiresAt}}<p style="color:#e53e3e;font-size:14px;line-height:1.5;margin:0 0 16px;">This invitation expires on {{.ExpiresAt}}.</p>{{end}}
    <p style="color:#a0aec0;font-size:13px;margin:0;">
      If you were not expecting this invitation you can safely ignore this email.
    </p>
  </td></tr>
  <tr><td style="background-color:#f8fafc;padding:24px 40px;text-align:center;border-top:1px solid #e2e8f0;">
    <p style="color:#a0aec0;font-size:12px;margin:0;">This email was sent by Centrora. Please do not reply to this email.</p>
  </td></tr>
</table>
</td></tr>
</table>
</body>
</html>',
    'Environment Invitation: {{.EnvName}}

{{.InviterName}} has invited you to the {{.EnvName}} environment in the {{.ProjectName}} project ({{.OrganizationName}}) with the role {{.Role}}.

Accept the invitation by clicking the link below:
{{.AcceptURL}}
{{if .ExpiresAt}}
This invitation expires on {{.ExpiresAt}}.
{{end}}
If you were not expecting this invitation you can safely ignore this email.',
    'go_template',
    TRUE
)
ON CONFLICT (email_type_id) WHERE app_id IS NULL DO NOTHING;

-- -------------------------------------------------------------------------
-- 3. Register migration
-- -------------------------------------------------------------------------
INSERT INTO schema_migrations (version, name, applied_at, success)
VALUES ('20260514_seed_centrora_invitation_email_types', 'Seed Centrora invitation email types and default templates', NOW(), true)
ON CONFLICT (version) DO NOTHING;
