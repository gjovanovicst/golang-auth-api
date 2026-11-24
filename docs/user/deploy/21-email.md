# Email and SMTP

The API sends verification links, password resets, 2FA codes and security notifications.
Without SMTP it still works: messages are printed to the log in development mode.

## How a template is resolved

```
per-application template (email_templates.app_id = this app)
        ↓ not found
global template (email_templates.app_id IS NULL)
        ↓ not found
hard-coded default in internal/email/defaults.go
```

The hard-coded defaults are an immutable safety net. Everything above them is editable in
the admin GUI (*Email Templates*), so a broken custom template never blocks login flows.

## Configuring a sender

1. Open *Admin GUI → Email Servers → New*.
2. Pick the scope: **global** (all applications) or a specific application.
3. Fill in host, port, username, password, from address and TLS mode.
4. Mark it **default** for the scope and **active**.
5. Send a test email from the same page.

Resolution order at send time: the application's default active sender → the application's
first active sender → the global default → the global first active sender. Per-application
senders are the right choice for multi-tenant deployments: each customer uses their own
SMTP identity.

## Template engines

| Engine | Use when |
|---|---|
| `go_template` | Default. Go `text/template` syntax: `{{.VerifyURL}}`, `{{if .UserName}}…{{end}}` |
| `placeholder` | Simple `{{variable}}` replacement, no logic |
| `raw_html` | The body is sent as-is; variables are not expanded |

Templates expose the variables registered for their email type. The editor shows the list
and autocompletes them; the same list is available from
`GET /admin/email-variables`.

## Email types

Types are a registry (`email_types`) that groups templates and declares their variables:

`email_verification`, `password_reset`, `two_fa_code`, `welcome`, `account_deactivated`,
`password_changed`, `magic_link`, `new_device_login`, `suspicious_activity`,
`api_key_expiring_soon`, `backup_email_verification`.

Each type has a default subject and a JSON list of variables (`name`, `description`,
`required`). Unknown variables render empty, so typos are silent — preview before saving.

## Development fallback

With no sender configured, `Send` logs the full message:

```
=== EMAIL (DEVELOPMENT/FALLBACK MODE) ===
To: user@example.com
Subject: Verify Your Email Address
Body (text): …
```

This is intentional: you can complete every flow locally without an SMTP server. The
verification link in the log can be pasted into the browser.

## Deliverability checklist

- Use a dedicated subdomain (e.g. `auth.example.com`) and publish SPF, DKIM and DMARC.
- Set the `From` address to that subdomain, not a consumer mailbox.
- Keep verification and reset links absolute and on the same domain users trust.
- If a link goes to a frontend page, configure the application's `frontend_url` and the
  link paths (`verify_email_path`, `reset_password_path`, `magic_link_path`).
- Never put secrets in a template; variables are stored in plain text in the database.

## Testing

```bash
# send a test email through the application's sender
curl -sX POST http://localhost:8080/admin/apps/$APP/email-test \
  -H "X-Admin-API-Key: $ADMIN_KEY" -H "Content-Type: application/json" \
  -d '{"to_email":"you@example.com"}'
```

The GUI has the same action behind *Email Servers → Test*. Delivery failures are logged
with the SMTP response; the API does not retry a failed send.
