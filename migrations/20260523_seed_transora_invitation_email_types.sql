-- Migration: Seed Transora invitation email type and default template
-- Date: 2026-05-23
-- Description: Adds the email type code used by Transora when delegating
--   invitations to Centrora:
--   transora_invitation – org-level invitation sent on behalf of Transora
-- Variables: inviter_name, organization_name, role, accept_url, expires_at

-- Section 1: email_types INSERT
INSERT INTO email_types (code, name, description, default_subject, variables, is_system, is_active) VALUES
(
    'transora_invitation',
    'Transora Organization Invitation',
    'Sent when a user is invited to join an organization in Transora.',
    'You''ve been invited to join {{organization_name}} on Transora',
    '[
        {"name": "inviter_name",      "description": "Name of the person who sent the invitation", "required": true},
        {"name": "organization_name", "description": "Name of the organization",                   "required": true},
        {"name": "role",              "description": "Role the invitee will receive",               "required": true},
        {"name": "accept_url",        "description": "URL to accept the invitation",                "required": true},
        {"name": "expires_at",        "description": "Invitation expiry date/time",                 "required": false}
    ]'::jsonb,
    TRUE,
    TRUE
)
ON CONFLICT (code) DO NOTHING;

-- Section 2: email_templates INSERT (default, app_id = NULL → applies to all apps)
INSERT INTO email_templates (app_id, email_type_id, name, subject, body_html, body_text, template_engine, is_active) VALUES
(
    NULL,
    (SELECT id FROM email_types WHERE code = 'transora_invitation'),
    'Default Transora Organization Invitation',
    'You''ve been invited to join {{.OrganizationName}} on Transora',
    '<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Transora Invitation</title>
</head>
<body style="margin:0;padding:0;background-color:#f4f7fa;font-family:-apple-system,BlinkMacSystemFont,''Segoe UI'',Roboto,''Helvetica Neue'',Arial,sans-serif;">
  <table width="100%" cellpadding="0" cellspacing="0" border="0" style="background-color:#f4f7fa;padding:40px 0;">
    <tr>
      <td align="center">
        <table width="600" cellpadding="0" cellspacing="0" border="0" style="background-color:#ffffff;border-radius:8px;box-shadow:0 2px 8px rgba(0,0,0,0.08);overflow:hidden;">

          <!-- Header -->
          <tr>
            <td style="background-color:#3b82f6;padding:32px 40px;">
              <h1 style="margin:0;color:#ffffff;font-size:24px;font-weight:600;letter-spacing:-0.3px;">Transora</h1>
            </td>
          </tr>

          <!-- Body -->
          <tr>
            <td style="padding:40px;">
              <h2 style="margin:0 0 16px 0;color:#1a1a2e;font-size:20px;font-weight:600;">You''ve been invited!</h2>
              <p style="margin:0 0 24px 0;color:#4a5568;font-size:15px;line-height:1.6;">
                <strong>{{.InviterName}}</strong> has invited you to join
                <strong>{{.OrganizationName}}</strong> on Transora as <strong>{{.Role}}</strong>.
              </p>
              <p style="margin:0 0 32px 0;color:#4a5568;font-size:15px;line-height:1.6;">
                Click the button below to accept the invitation and get started.
              </p>

              <!-- CTA button -->
              <table cellpadding="0" cellspacing="0" border="0" style="margin:0 0 32px 0;">
                <tr>
                  <td style="border-radius:6px;background-color:#3b82f6;">
                    <a href="{{.AcceptURL}}"
                       style="display:inline-block;padding:14px 32px;color:#ffffff;font-size:15px;font-weight:600;text-decoration:none;border-radius:6px;">
                      Accept Invitation
                    </a>
                  </td>
                </tr>
              </table>

              <!-- Fallback URL -->
              <p style="margin:0 0 8px 0;color:#718096;font-size:13px;">
                Or copy this link into your browser:
              </p>
              <p style="margin:0 0 24px 0;word-break:break-all;">
                <a href="{{.AcceptURL}}" style="color:#3b82f6;font-size:13px;">{{.AcceptURL}}</a>
              </p>

              {{if .ExpiresAt}}
              <p style="margin:0 0 24px 0;color:#e53e3e;font-size:13px;">
                This invitation expires on {{.ExpiresAt}}.
              </p>
              {{end}}

              <p style="margin:0;color:#a0aec0;font-size:13px;line-height:1.5;">
                If you were not expecting this invitation you can safely ignore this email.
              </p>
            </td>
          </tr>

          <!-- Footer -->
          <tr>
            <td style="background-color:#f8fafc;padding:20px 40px;border-top:1px solid #e2e8f0;">
              <p style="margin:0;color:#a0aec0;font-size:12px;text-align:center;">
                This email was sent by Transora. Please do not reply to this email.
              </p>
            </td>
          </tr>

        </table>
      </td>
    </tr>
  </table>
</body>
</html>',
    'You''ve been invited to join {{.OrganizationName}} on Transora

{{.InviterName}} has invited you to join {{.OrganizationName}} as {{.Role}}.

Accept the invitation here:
{{.AcceptURL}}

{{if .ExpiresAt}}This invitation expires on {{.ExpiresAt}}.{{end}}

If you were not expecting this invitation you can safely ignore this email.

-- Transora',
    'go_template',
    TRUE
)
ON CONFLICT (email_type_id) WHERE app_id IS NULL DO NOTHING;

-- Section 3: migration record
INSERT INTO schema_migrations (version, name, applied_at, success)
VALUES ('20260523_seed_transora_invitation_email_types', 'Seed Transora invitation email type and default template', NOW(), true)
ON CONFLICT (version) DO NOTHING;
