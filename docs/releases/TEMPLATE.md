# vX.Y.Z — YYYY-MM-DD

**One sentence: the theme of this release.**

<!-- Two or three sentences of context. What changed for the reader? -->

## Highlights

### <Area>

- **Feature** — what it does and why it matters.
- **Fix** — what was broken and what the user sees now.

<!-- Repeat per area: Authentication, 2FA, Multi-tenancy, OIDC, Operations, Documentation. -->

## Install

```bash
docker pull ghcr.io/gjovanovicst/golang-auth-api:X.Y.Z
```

Archives: `auth-api_X.Y.Z_{linux,darwin}_{amd64,arm64}.tar.gz`,
`auth-api_X.Y.Z_windows_amd64.zip`, plus `checksums.txt`.

## Verify what you run

```bash
cosign verify \
  --certificate-identity-regexp "^https://github.com/gjovanovicst/golang-auth-api/" \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  ghcr.io/gjovanovicst/golang-auth-api@<digest>
```

## Breaking changes

<!-- "None" for patch releases. For a breaking change: what breaks, who is affected, and
the migration path. Link docs/BREAKING_CHANGES.md. -->

## Upgrade

1. Back up the database.
2. Apply migrations (`make migrate-up`).
3. Pull the new image and restart.
4. Verify `/health`, a login (with 2FA) and the admin GUI.

## Security

<!-- Fixes and hardening included in this release. Credit reporters when they agreed. -->

## Thanks

<!-- Contributors, reporters, reviewers. -->
