-- Migration: Seed Permissio invitation email type and default template
-- Date: 2026-05-15
-- Description: Adds the email type code used by Permissio when delegating
--   invitations to Centrora:
--   permissio_invitation – org-level invitation sent on behalf of Permissio
-- Variables: inviter_name, organization_name, role, accept_url, expires_at

-- -------------------------------------------------------------------------
-- 1. Email type
-- -------------------------------------------------------------------------

INSERT INTO email_types (code, name, description, default_subject, variables, is_system, is_active) VALUES
(
    'permissio_invitation',
    'Permissio Organization Invitation',
    'Sent when a user is invited to join an organization in Permissio.',
    'You''ve been invited to join {{organization_name}} on Permissio',
    '[{"name": "inviter_name",      "description": "Name or email of the person who sent the invite", "required": true},
      {"name": "organization_name", "description": "Name of the organization",                        "required": true},
      {"name": "role",              "description": "Role the invitee will receive",                   "required": true},
      {"name": "accept_url",        "description": "URL the invitee clicks to accept the invitation", "required": true},
      {"name": "expires_at",        "description": "Human-readable expiry date/time of the invite",   "required": false}]'::jsonb,
    TRUE, TRUE
)
ON CONFLICT (code) DO NOTHING;

-- -------------------------------------------------------------------------
-- 2. Default global template
-- -------------------------------------------------------------------------

INSERT INTO email_templates (app_id, email_type_id, name, subject, body_html, body_text, template_engine, is_active) VALUES
(
    NULL,
    (SELECT id FROM email_types WHERE code = 'permissio_invitation'),
    'Default Permissio Organization Invitation',
    'You''ve been invited to join {{.OrganizationName}} on Permissio',
    '<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Permissio Invitation</title>
</head>
<body style="margin:0;padding:0;background-color:#f4f7fa;font-family:-apple-system,BlinkMacSystemFont,''Segoe UI'',Roboto,''Helvetica Neue'',Arial,sans-serif;">
<table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="background-color:#f4f7fa;padding:40px 0;">
<tr><td align="center">
<table role="presentation" width="600" cellspacing="0" cellpadding="0" style="background-color:#ffffff;border-radius:8px;box-shadow:0 2px 8px rgba(0,0,0,0.08);overflow:hidden;">
  <tr><td style="background-color:#6366f1;padding:32px 40px;text-align:center;">
    <h1 style="color:#ffffff;margin:0;font-size:24px;font-weight:600;">Permissio</h1>
  </td></tr>
  <tr><td style="padding:40px;">
    <h2 style="color:#1a1a2e;margin:0 0 16px;font-size:20px;">You''ve been invited!</h2>
    <p style="color:#4a5568;font-size:16px;line-height:1.6;margin:0 0 24px;">
      <strong>{{.InviterName}}</strong> has invited you to join <strong>{{.OrganizationName}}</strong> on Permissio as <strong>{{.Role}}</strong>.
    </p>
    <table role="presentation" cellspacing="0" cellpadding="0" style="margin:0 auto 24px;">
    <tr><td style="background-color:#6366f1;border-radius:6px;">
      <a href="{{.AcceptURL}}" style="display:inline-block;padding:14px 32px;color:#ffffff;text-decoration:none;font-size:16px;font-weight:600;">Accept Invitation</a>
    </td></tr>
    </table>
    <p style="color:#718096;font-size:14px;line-height:1.5;margin:0 0 8px;">
      If the button doesn''t work, copy and paste this link into your browser:
    </p>
    <p style="color:#6366f1;font-size:14px;word-break:break-all;margin:0 0 24px;">{{.AcceptURL}}</p>
    {{if .ExpiresAt}}<p style="color:#e53e3e;font-size:14px;line-height:1.5;margin:0 0 16px;">This invitation expires on {{.ExpiresAt}}.</p>{{end}}
    <p style="color:#a0aec0;font-size:13px;margin:0;">
      If you were not expecting this invitation, you can safely ignore this email.
    </p>
  </td></tr>
  <tr><td style="background-color:#f8fafc;padding:24px 40px;text-align:center;border-top:1px solid #e2e8f0;">
    <p style="color:#a0aec0;font-size:12px;margin:0;">This email was sent by Permissio. Please do not reply to this email.</p>
  </td></tr>
</table>
</td></tr>
</table>
</body>
</html>',
    'You''ve been invited to join {{.OrganizationName}} on Permissio

{{.InviterName}} has invited you to join {{.OrganizationName}} on Permissio as {{.Role}}.

Accept your invitation by visiting the link below:
{{.AcceptURL}}
{{if .ExpiresAt}}
This invitation expires on {{.ExpiresAt}}.
{{end}}
If you were not expecting this invitation, you can safely ignore this email.',
    'go_template',
    TRUE
)
ON CONFLICT (email_type_id) WHERE app_id IS NULL DO NOTHING;

-- -------------------------------------------------------------------------
-- 3. Register migration
-- -------------------------------------------------------------------------
INSERT INTO schema_migrations (version, name, applied_at, success)
VALUES ('20260515_seed_permissio_invitation_email_types', 'Seed Permissio invitation email type and default template', NOW(), true)
ON CONFLICT (version) DO NOTHING;
