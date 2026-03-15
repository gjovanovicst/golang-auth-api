---
name: email-system
description: Complete documentation of the email subsystem including template resolution, variable pipeline, rendering engines, SMTP config, and default templates.
license: MIT
---

## Architecture

The email system is in `internal/email/` and follows a multi-layered pipeline:

```
Service (orchestrator)
  |-- VariableResolver  -> resolves template variables from 4 sources
  |-- Renderer          -> renders templates with 3 supported engines
  |-- Sender            -> sends via SMTP (or logs in dev mode)
  |-- Repository        -> DB access for types, templates, server configs
```

Entry point: `email.Service.SendEmailWithContext(appID, emailTypeCode, toEmail, userID, vars)`

## Send Pipeline

```
1. VariableResolver.ResolveVariables()  -- build final variable map
2. resolveTemplate()                     -- find the right template
3. Renderer.RenderTemplate()             -- render subject, HTML, text
4. resolveSMTPConfigForTemplate()        -- find the right SMTP config
5. Sender.Send()                         -- deliver via SMTP
```

## Template Resolution Chain

Defined in `internal/email/service.go` method `resolveTemplate`:

```
1. DB: app-specific template
   WHERE app_id = ? AND email_type_id = ? AND is_active = true

2. DB: global default template
   WHERE app_id IS NULL AND email_type_id = ? AND is_active = true

3. Hardcoded fallback
   defaults.go -> GetDefaultTemplate(typeCode)
```

## SMTP Resolution Chain

Defined in `internal/email/service.go` method `resolveSMTPConfig`:

```
1. DB: per-app config
   WHERE app_id = ? AND is_active = true AND is_default = true

2. DB: global config
   WHERE app_id IS NULL AND is_active = true AND is_default = true

3. Empty config -> dev/fallback mode (log to stdout)
```

With template override (`resolveSMTPConfigForTemplate`):
```
1. If template has ServerConfigID -> use that specific config by ID
2. Otherwise -> standard chain above
3. Apply template-level FromEmail/FromName overrides on top
```

## Variable Resolution Pipeline

Defined in `internal/email/resolver.go`. Resolution priority (lowest to highest):

| Layer | Source | Method | Example Variables |
|-------|--------|--------|-------------------|
| 1 (lowest) | Static defaults | `applyStaticDefaults` | DefaultValue from email_types.variables JSONB |
| 2 | App/system settings | `applySettingsVars` | `app_name`, `frontend_url` |
| 3 | User profile | `applyUserVars` | `user_email`, `user_name`, `first_name`, `last_name`, `locale`, `profile_picture` |
| Always | Fallback | -- | `user_email` = toEmail if not set |
| 4 (highest) | Explicit caller vars | direct map copy | `verification_link`, `code`, `reset_link`, etc. |

### Variable Sources

| Source | Variables | How Resolved |
|--------|-----------|-------------|
| `user` (auto) | `user_email`, `user_name`, `first_name`, `last_name`, `locale`, `profile_picture` | From DB user record |
| `setting` (auto) | `app_name`, `frontend_url` | app name from DB/env, frontend_url from env |
| `explicit` (caller) | `verification_link`, `verification_token`, `reset_link`, `code`, `expiration_minutes`, `change_time`, `magic_link` | Must be passed by calling code |

All variable constants are in `internal/email/types.go`.

## Template Engines

Defined in `internal/email/renderer.go`:

| Engine | Constant | Syntax | HTML Escaping |
|--------|----------|--------|---------------|
| Go Template | `go_template` | `{{.AppName}}` or `{{.app_name}}` | Yes (automatic) |
| Placeholder | `placeholder` | `{app_name}` | No |
| Raw HTML | `raw_html` | `{{.AppName}}` | No (unsafe) |

The renderer converts all variables to both `snake_case` and `PascalCase` forms, so templates can use either style.

Subject lines always use Go template syntax regardless of engine.

## Email Types (7 built-in)

Constants in `internal/email/types.go`:

| Code | Constant | Variables Used |
|------|----------|----------------|
| `email_verification` | TypeEmailVerification | app_name, verification_link, user_email |
| `password_reset` | TypePasswordReset | app_name, reset_link, expiration_minutes |
| `two_fa_code` | TypeTwoFACode | app_name, code, expiration_minutes |
| `welcome` | TypeWelcome | app_name, user_email |
| `account_deactivated` | TypeAccountDeactivated | app_name, user_email |
| `password_changed` | TypePasswordChanged | app_name, change_time, user_email |
| `magic_link` | TypeMagicLink | app_name, magic_link, expiration_minutes |

## Default Templates

Defined in `internal/email/defaults.go`. All use `go_template` engine, provide both HTML and plain text bodies, and use responsive table-based layout with consistent styling (indigo `#4f46e5` primary color, `600px` max width).

## Service Methods

### Typed Send Methods (in `internal/email/service.go`)

| Method | Email Type | Extra Vars |
|--------|-----------|------------|
| `SendVerificationEmail` | email_verification | verification_link (built from FRONTEND_URL + token) |
| `SendPasswordResetEmail` | password_reset | reset_link, expiration_minutes=60 |
| `Send2FACodeEmail` | two_fa_code | code, expiration_minutes=5 |
| `SendWelcomeEmail` | welcome | (none) |
| `SendAccountDeactivatedEmail` | account_deactivated | (none) |
| `SendPasswordChangedEmail` | password_changed | change_time |
| `SendMagicLinkEmail` | magic_link | magic_link, expiration_minutes=10 |
| `SendAdmin2FACodeEmail` | N/A (hardcoded HTML) | Bypasses app-scoped resolution |
| `SendAdminMagicLinkEmail` | N/A (hardcoded HTML) | Bypasses app-scoped resolution |

### Management Methods

Template CRUD: `GetTemplatesByApp`, `GetGlobalDefaultTemplates`, `GetTemplateByID`, `SaveAppTemplate`, `SaveGlobalTemplate`, `DeleteTemplate`, `ResetTemplateToDefault`, `PreviewTemplate`

Email type CRUD: `GetAllEmailTypes`, `GetEmailTypeByCode`, `CreateEmailType`, `UpdateEmailType`, `DeleteEmailType`

Server config CRUD: `GetServerConfig`, `SaveServerConfig`, `DeleteServerConfig`, `GetAllServerConfigs`, etc.

## SMTP Sending (`internal/email/sender.go`)

Uses `gopkg.in/mail.v2`.

**Dev mode:** If SMTP host is empty or `"smtp.example.com"`, logs email to stdout (no error).

**TLS handling:**
- Port 465: implicit SSL (`d.SSL = true`)
- Other ports: `MandatoryStartTLS`, MinVersion TLS 1.2

**Error behavior:**
- `Send()`: on SMTP failure, logs email as fallback and returns nil (email failure doesn't break business flow)
- `SendTest()`: always returns errors (for admin testing)

## Database Models

- `EmailType` (`pkg/models/email_type.go`) -- types with variable definitions as JSONB
- `EmailTemplate` (`pkg/models/email_template.go`) -- templates scoped to app or global
- `EmailServerConfig` (`pkg/models/email_server_config.go`) -- SMTP configs scoped to app or global

## When To Use This Skill

Load this skill when working on email sending, email templates, SMTP configuration, template variables, or any email-related feature.
