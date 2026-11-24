# Overview

The Auth API is a self-hosted authentication and authorization service written in Go.
It gives your applications a complete identity layer — credentials, social login,
two-factor, sessions, roles and an OIDC provider — behind one HTTP API and one
PostgreSQL database.

It is a **service**, not a library: your frontend talks to it over HTTP, and the
admin GUI lets you manage tenants, applications, users, roles and email templates
without touching SQL.

## Who it is for

- **SaaS teams** that need multi-tenant auth (one tenant → many applications) without
  building it from scratch.
- **Self-hosters** who want their identity data in their own PostgreSQL, not in a
  third-party cloud.
- **Projects that need standards**: OAuth2 social login, OpenID Connect, TOTP/WebAuthn
  two-factor, and JWT access tokens with refresh rotation.

## What is included

| Area | Capabilities |
|---|---|
| Credentials | Email + password registration, login, password policies per application (length, complexity, history, max age), password reset, email verification |
| Tokens | JWT access tokens, rotating refresh tokens, Redis-backed blacklisting, per-application TTL overrides |
| Social | Google, Facebook and GitHub OAuth2, account linking, merge confirmation, signed single-use `state` |
| Two-factor | TOTP, email codes, SMS codes, backup email, passkeys (WebAuthn), recovery codes, trusted devices, SMS recovery fallback |
| Passwordless | Magic-link login and discoverable passkeys |
| Multi-tenancy | Tenants → applications → users, per-app branding, password policy, 2FA policy and token TTLs |
| Authorization | RBAC roles and permissions per application; admin API keys and per-app API keys |
| Sessions | Server-side session registry in Redis, list/revoke endpoints, session groups for cross-app SSO |
| OIDC | Full provider: discovery, JWKS, authorize, token, userinfo, introspect, revoke, RP-initiated logout |
| Operations | Admin GUI (HTMX), webhooks, activity logs with anomaly detection, health checks, Prometheus metrics |

## How it is put together

```
        your frontend / mobile app
                   │  HTTPS + X-App-ID
                   ▼
        ┌──────────────────────┐        ┌──────────────┐
        │   Auth API (Go/Gin)  │───────▶│  PostgreSQL  │  users, tenants, apps,
        │  handlers → service  │        │              │  roles, templates, logs
        │     → repository     │        └──────────────┘
        └──────────┬───────────┘
                   │ sessions, tokens, 2FA codes, rate limits
                   ▼
             ┌───────────┐
             │   Redis   │
             └───────────┘
```

Every application is identified by `X-App-ID`. The same user email can exist in two
applications and they are different users — isolation is enforced in every query.

## Technology

| Component | Choice |
|---|---|
| Language | Go (1.23+) |
| HTTP | Gin |
| Database | PostgreSQL with GORM + SQL migrations |
| Cache / sessions | Redis |
| Admin UI | Server-rendered Go templates + HTMX |
| API docs | Swagger/OpenAPI generated from annotations |

## Next steps

- [Quickstart](01-quickstart.md) — a running instance in five minutes.
- [Concepts](02-concepts.md) — tenants, applications, tokens and sessions.
- [Authentication](10-authentication.md) — the integration surface.
