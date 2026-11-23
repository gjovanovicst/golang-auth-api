# Roadmap

What the project ships, what is planned next, and how work is prioritised.
This is a direction, not a contract — scope can change with security and
compatibility work taking precedence.

## Principles

1. **Security before features.** Anything that weakens the auth path is fixed or
   reverted, even if it delays the roadmap.
2. **Compatible within a major version.** Additive changes are preferred;
   breaking changes are documented in `docs/BREAKING_CHANGES.md` and announced a
   minor version ahead.
3. **Documented or it does not exist.** Every endpoint, environment variable and
   admin screen is documented in `docs/` and, from v1.0.0 on, on the public site.
4. **Small, reviewable changes.** One PR = one concern; `main` stays releasable.

## Status legend

| Symbol | Meaning |
|---|---|
| ✅ | Shipped |
| 🚧 | In progress |
| 📋 | Planned |
| 💡 | Under consideration — needs a decision |

## v1.0.0 — Stabilization

The first public release. The product surface is complete; the focus is on
distribution, documentation and repeatable operations.

**Shipped**

- ✅ Password auth: registration, login, refresh-token rotation, logout, JWT
  blacklisting
- ✅ Email verification, password reset, password policies per application
  (length, complexity, history, max age)
- ✅ Social login: Google, Facebook, GitHub, with account linking and merge flow
- ✅ 2FA: TOTP, email, SMS, backup email, passkeys; recovery codes and trusted
  devices
- ✅ SMS recovery fallback (one-time code when the primary factor is unavailable)
- ✅ Magic-link login and passwordless passkeys
- ✅ Multi-tenancy: tenants → applications → users, per-app branding and settings
- ✅ RBAC: roles, permissions, per-app assignments
- ✅ Admin GUI (HTMX) for tenants, applications, users, roles, email templates,
  webhooks, sessions, settings and monitoring
- ✅ OIDC provider (discovery, authorize, token, userinfo, JWKS, revocation) and
  SSO session groups with cross-app logout
- ✅ Webhooks, activity logs with anomaly detection, session management
- ✅ Swagger/OpenAPI generation, Docker and compose deployment

**Remaining for 1.0.0**

- 🚧 Public documentation site: user guide (integrate + deploy tracks) and API
  reference built from the OpenAPI spec
- 🚧 Release automation: signed multi-arch images, binaries with checksums,
  SBOM, release notes per tag
- 📋 Quality gates: `make check` aggregate, OpenAPI drift check, end-to-end smoke
  test, coverage threshold
- 📋 Operations docs: runbook, upgrade guide, backup/restore procedure

## v1.1 — Integrations

- 📋 SDK/sample clients (Go, TypeScript) and a Postman/Insomnia collection
- 📋 Webhook delivery hardening: retry policy UI, dead-letter inspection, HMAC
  signature helpers in docs
- 📋 Email deliverability: bounce handling, per-app sender verification guidance
- 💡 SCIM 2.0 provisioning (users/groups) over the existing RBAC model

## v1.2 — Enterprise operations

- 📋 Per-application rate-limit policies (currently global defaults)
- 📋 Audit log export (CSV/JSONL) and retention policy presets
- 📋 OIDC client metadata import/export and key rotation runbook
- 💡 Organisation/team model above tenants
- 💡 Configurable password policy presets (NIST, PCI-DSS)

## v2.0 — Next major

Reserved for changes that cannot be additive: token format migration, a versioned
API surface (`/v2`), or a storage schema redesign. Not designed yet.

## Patch releases

Security and correctness fixes ship as patch releases from `release/x.y` branches
using the `backport/x.y` label on the merged PR. Patch releases never contain new
features and are always documented in `CHANGELOG.md`.

## How work is tracked

- Bugs and feature requests use the issue forms in `.github/ISSUE_TEMPLATE/`.
- Labels: `area:*` (where), `p0`–`p2` (priority), `triage` → `ready` (groomed),
  `breaking`, `backport/x.y`, `skip-changelog`.
- A change lands only through a PR that passes `Test`, `Build`, `Security Scan`
  and the `changelog` check.
